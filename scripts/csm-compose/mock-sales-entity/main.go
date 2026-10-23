// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//

// LOCAL DEV ONLY. A stand-in for REST sales/sales-entity-service with the
// handful of endpoints entity-service's portal membership writes use, plus a
// switch that makes it answer 503 to everything -- which is how the deferred
// Salesforce write (entity-service SALESFORCE_DEFERRED_WRITES_ENABLED) is
// exercised end to end without Salesforce:
//
//	POST /__mock/outage {"down":true|false}   flip the outage
//	GET  /__mock/state                        dump contacts and memberships
//
// With MOCK_SF_WRITES_BROKER set it also plays sales-entity-service's
// deferred-write CONSUMER (digiops-sales membershipwrite): it reads
// salesforce.membership_write.requested from MOCK_SF_WRITES_TOPIC, performs
// the same search-first write against its own store, retries in place every
// MOCK_SF_WRITES_RETRY while the outage is on (up to MOCK_SF_WRITES_MAX_ATTEMPTS,
// then reports a non-terminal failure), and publishes
// salesforce.membership_write.completed / .failed to MOCK_SF_RESULTS_TOPIC --
// the same contract the real consumer honours, so entity-service's side of
// the flow is exercised end to end without Salesforce or MySQL.
//
// Records live in memory and vanish on restart. Ids are invented
// ("003MOCK…", "a0eMOCK…") and never collide with real Salesforce ids.
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	kafka "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

type contact struct {
	ID                  string          `json:"id"`
	Email               *string         `json:"email"`
	Name                *string         `json:"name"`
	FirstName           *string         `json:"firstName"`
	LastName            *string         `json:"lastName"`
	IsCsAdmin           *bool           `json:"isCsAdmin"`
	IsCsIntegrationUser *bool           `json:"isCsIntegrationUser"`
	LockoutStatus       *bool           `json:"lockoutStatus"`
	Account             *contactAccount `json:"account"`
	LastModifiedDate    *string         `json:"lastModifiedDate"`
}

type contactAccount struct {
	ID             *string `json:"id"`
	Classification *string `json:"classification"`
	IsPartner      *bool   `json:"isPartner"`
}

type projectContact struct {
	ID               string        `json:"id"`
	Email            string        `json:"email"`
	State            *string       `json:"state"`
	Roles            []string      `json:"roles"`
	Type             *string       `json:"type"`
	Contact          *pcContact    `json:"contact"`
	Subscription     *subscription `json:"subscription"`
	CreatedDate      *string       `json:"createdDate"`
	LastModifiedDate *string       `json:"lastModifiedDate"`
}

type pcContact struct {
	ID         *string `json:"id"`
	Name       *string `json:"name"`
	Email      *string `json:"email"`
	CustomerID *string `json:"customerId"`
}

type subscription struct {
	ID         *string `json:"id"`
	Name       *string `json:"name"`
	Key        *string `json:"key"`
	CustomerID *string `json:"customerId"`
}

type customer struct {
	ID                    string  `json:"id"`
	Name                  *string `json:"name"`
	AccountClassification *string `json:"accountClassification"`
	DomainList            *string `json:"domainList"`
	IsPartner             *bool   `json:"isPartner"`
	LastModifiedDate      *string `json:"lastModifiedDate"`
}

type store struct {
	mu          sync.Mutex
	down        atomic.Bool
	seq         int
	contacts    map[string]*contact
	memberships map[string]*projectContact
	// domainList is what every customer-search answers with, so the
	// invitation validator accepts the test address (MOCK_SF_DOMAIN_LIST).
	domainList string
	calls      atomic.Int64
}

func str(s string) *string { return &s }
func boolp(b bool) *bool   { return &b }
func now() *string         { return str(time.Now().UTC().Format(time.RFC3339Nano)) }

// idEpoch makes ids unique across restarts, as Salesforce ids are: the store
// is in memory, and a sequence that restarted at 1 would hand a new record an
// id an earlier run's CSM row already carries, so the upsert would match the
// wrong row. 3-char prefix + "M" + 8 digits of epoch + 6 of sequence = 18.
var idEpoch = time.Now().Unix() % 100000000

func (s *store) nextID(prefix string) string {
	s.seq++
	return fmt.Sprintf("%sM%08d%06d", prefix, idEpoch, s.seq)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// outageGate answers 503 to every business endpoint while the outage is on.
func (s *store) outageGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/__mock/") || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		s.calls.Add(1)
		if s.down.Load() {
			log.Printf("OUTAGE  %s %s -> 503", r.Method, r.URL.Path)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "mock sales-entity: simulated outage"})
			return
		}
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9200"
	}
	s := &store{contacts: map[string]*contact{}, memberships: map[string]*projectContact{}, domainList: os.Getenv("MOCK_SF_DOMAIN_LIST")}
	if s.domainList == "" {
		s.domainList = "example.com"
	}
	if os.Getenv("MOCK_SF_START_DOWN") == "true" {
		s.down.Store(true)
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /__mock/outage", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Down bool `json:"down"`
		}
		if err := decode(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.down.Store(body.Down)
		log.Printf("outage set to %v", body.Down)
		writeJSON(w, http.StatusOK, map[string]any{"down": body.Down})
	})
	mux.HandleFunc("GET /__mock/state", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"down": s.down.Load(), "calls": s.calls.Load(), "contacts": s.contacts, "memberships": s.memberships})
	})

	// OAuth2 client credentials: any client is welcome.
	mux.HandleFunc("POST /oauth2/token", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"access_token": "mock-sales-entity-token", "token_type": "Bearer", "expires_in": 3600})
	})

	mux.HandleFunc("POST /customer-search", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs []string `json:"ids"`
		}
		_ = decode(r, &req)
		out := make([]customer, 0, len(req.IDs))
		for _, id := range req.IDs {
			out = append(out, customer{ID: id, Name: str("Mock Account " + id), AccountClassification: str("Customer"),
				DomainList: str(s.domainList), IsPartner: boolp(false), LastModifiedDate: now()})
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /contacts/search", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Limit int    `json:"limit"`
		}
		_ = decode(r, &req)
		s.mu.Lock()
		defer s.mu.Unlock()
		out := []contact{}
		for _, c := range s.contacts {
			if (req.ID != "" && c.ID == req.ID) || (req.Email != "" && c.Email != nil && strings.EqualFold(*c.Email, req.Email)) {
				out = append(out, *c)
			}
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /contacts", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			FirstName           string `json:"firstName"`
			LastName            string `json:"lastName"`
			Email               string `json:"email"`
			AccountID           string `json:"accountId"`
			IsCsIntegrationUser bool   `json:"isCsIntegrationUser"`
		}
		if err := decode(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		c := &contact{ID: s.nextID("003"), Email: str(strings.ToLower(in.Email)), Name: str(strings.TrimSpace(in.FirstName + " " + in.LastName)),
			FirstName: str(in.FirstName), LastName: str(in.LastName), IsCsAdmin: boolp(false), IsCsIntegrationUser: boolp(in.IsCsIntegrationUser),
			LockoutStatus: boolp(true), Account: &contactAccount{ID: str(in.AccountID), Classification: str("Customer"), IsPartner: boolp(false)},
			LastModifiedDate: now()}
		s.contacts[c.ID] = c
		// Like Salesforce: every POST creates, duplicates included.
		writeJSON(w, http.StatusCreated, c)
	})

	mux.HandleFunc("PATCH /contacts/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			LockoutStatus bool `json:"lockoutStatus"`
		}
		_ = decode(r, &in)
		s.mu.Lock()
		defer s.mu.Unlock()
		c, ok := s.contacts[r.PathValue("id")]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such contact"})
			return
		}
		c.LockoutStatus = boolp(in.LockoutStatus)
		c.LastModifiedDate = now()
		writeJSON(w, http.StatusOK, c)
	})

	mux.HandleFunc("POST /project-contacts/search", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID        string `json:"id"`
			ProjectID string `json:"projectId"`
			ContactID string `json:"contactId"`
		}
		_ = decode(r, &req)
		s.mu.Lock()
		defer s.mu.Unlock()
		out := []projectContact{}
		for _, m := range s.memberships {
			switch {
			case req.ID != "" && m.ID == req.ID:
				out = append(out, *m)
			case req.ID == "" && req.ProjectID != "" && req.ContactID != "" &&
				m.Subscription != nil && m.Subscription.ID != nil && *m.Subscription.ID == req.ProjectID &&
				m.Contact != nil && m.Contact.ID != nil && *m.Contact.ID == req.ContactID:
				out = append(out, *m)
			}
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /project-contacts", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ProjectID string   `json:"projectId"`
			ContactID string   `json:"contactId"`
			State     string   `json:"state"`
			Role      []string `json:"role"`
		}
		if err := decode(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		c, ok := s.contacts[in.ContactID]
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown contactId " + in.ContactID})
			return
		}
		email := ""
		if c.Email != nil {
			email = *c.Email
		}
		var accountID *string
		if c.Account != nil {
			accountID = c.Account.ID
		}
		m := &projectContact{ID: s.nextID("a0e"), Email: email, State: str(in.State), Roles: in.Role, Type: str("OWN CONTACT"),
			Contact:      &pcContact{ID: str(c.ID), Name: c.Name, Email: c.Email, CustomerID: accountID},
			Subscription: &subscription{ID: str(in.ProjectID), Name: str("Mock Project"), Key: str("MOCKPROJ"), CustomerID: accountID},
			CreatedDate:  now(), LastModifiedDate: now()}
		s.memberships[m.ID] = m
		writeJSON(w, http.StatusCreated, m)
	})

	mux.HandleFunc("PATCH /project-contacts/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			State *string   `json:"state"`
			Role  *[]string `json:"role"`
		}
		_ = decode(r, &in)
		s.mu.Lock()
		defer s.mu.Unlock()
		m, ok := s.memberships[r.PathValue("id")]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such membership"})
			return
		}
		if in.State != nil {
			m.State = in.State
		}
		if in.Role != nil {
			m.Roles = *in.Role
		}
		m.LastModifiedDate = now()
		writeJSON(w, http.StatusOK, m)
	})

	if broker := os.Getenv("MOCK_SF_WRITES_BROKER"); broker != "" {
		go s.consumeWrites(context.Background(), broker)
	}
	log.Printf("mock sales-entity listening on :%s (down=%v, domainList=%s)", port, s.down.Load(), s.domainList)
	log.Fatal(http.ListenAndServe(":"+port, s.outageGate(mux)))
}

// ---- the deferred-write consumer ---------------------------------------

type envelope struct {
	Type     string          `json:"type"`
	EntityID string          `json:"entityId"`
	Payload  json.RawMessage `json:"payload"`
}

type requested struct {
	RequestID           string   `json:"requestId"`
	ProjectContactID    string   `json:"projectContactId"`
	ProjectSfID         string   `json:"projectSfId"`
	AccountSfID         string   `json:"accountSfId"`
	Email               string   `json:"email"`
	FirstName           string   `json:"firstName"`
	LastName            string   `json:"lastName"`
	Roles               []string `json:"roles"`
	State               string   `json:"state"`
	IsCsIntegrationUser bool     `json:"isCsIntegrationUser"`
}

type completed struct {
	RequestID         string   `json:"requestId"`
	ProjectContactID  string   `json:"projectContactId"`
	ContactSfID       string   `json:"contactSfId"`
	MembershipSfID    string   `json:"membershipSfId"`
	State             string   `json:"state"`
	Roles             []string `json:"roles"`
	ContactCreated    bool     `json:"contactCreated"`
	MembershipCreated bool     `json:"membershipCreated"`
	LastModifiedDate  string   `json:"lastModifiedDate,omitempty"`
}

type failed struct {
	RequestID        string `json:"requestId"`
	ProjectContactID string `json:"projectContactId"`
	Reason           string `json:"reason"`
	Terminal         bool   `json:"terminal"`
}

var errOutage = errors.New("simulated outage")

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(os.Getenv(key)); err == nil && d > 0 {
		return d
	}
	return def
}

// consumeWrites is the mock stand-in for sales-entity-service's consumer.
func (s *store) consumeWrites(ctx context.Context, broker string) {
	topic := envOr("MOCK_SF_WRITES_TOPIC", "salesforce-writes")
	results := envOr("MOCK_SF_RESULTS_TOPIC", "salesforce-write-results")
	group := envOr("MOCK_SF_WRITES_GROUP", "mock-sales-entity-salesforce-writes")
	retry := envDuration("MOCK_SF_WRITES_RETRY", 3*time.Second)
	maxAttempts := envInt("MOCK_SF_WRITES_MAX_ATTEMPTS", 20)
	dialer := &kafka.Dialer{TLS: &tls.Config{MinVersion: tls.VersionTLS12}, SASLMechanism: plain.Mechanism{Username: "$ConnectionString", Password: os.Getenv("MOCK_SF_WRITES_CONNECTION_STRING")}}
	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: []string{broker}, GroupID: group, Topic: topic, Dialer: dialer, StartOffset: kafka.FirstOffset, WatchPartitionChanges: true})
	writer := &kafka.Writer{Addr: kafka.TCP(broker), Topic: results, Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll,
		Transport: &kafka.Transport{TLS: &tls.Config{MinVersion: tls.VersionTLS12}, SASL: dialer.SASLMechanism}}
	log.Printf("CONSUMER reading %s (group %s), answering on %s, retry %s up to %d attempts", topic, group, results, retry, maxAttempts)
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			log.Printf("CONSUMER fetch: %v", err)
			time.Sleep(time.Second)
			continue
		}
		var env envelope
		var cmd requested
		if json.Unmarshal(msg.Value, &env) != nil || env.Type != "salesforce.membership_write.requested" || json.Unmarshal(env.Payload, &cmd) != nil {
			log.Printf("CONSUMER skipping record at offset %d", msg.Offset)
			_ = reader.CommitMessages(ctx, msg)
			continue
		}
		if cmd.RequestID == "" {
			cmd.RequestID = env.EntityID
		}
		var result any
		var resultType string
		for attempt := 1; ; attempt++ {
			done, err := s.write(cmd)
			if err == nil {
				result, resultType = done, "salesforce.membership_write.completed"
				log.Printf("CONSUMER request %s written on attempt %d: contact %s membership %s", cmd.RequestID, attempt, done.ContactSfID, done.MembershipSfID)
				break
			}
			if attempt >= maxAttempts {
				result, resultType = failed{RequestID: cmd.RequestID, ProjectContactID: cmd.ProjectContactID, Reason: "retries exhausted: " + err.Error(), Terminal: false}, "salesforce.membership_write.failed"
				log.Printf("CONSUMER request %s gave up after %d attempts: %v", cmd.RequestID, attempt, err)
				break
			}
			log.Printf("CONSUMER request %s attempt %d failed (%v); retrying in %s", cmd.RequestID, attempt, err, retry)
			time.Sleep(retry)
		}
		raw, _ := json.Marshal(result)
		body, _ := json.Marshal(envelope{Type: resultType, EntityID: cmd.RequestID, Payload: raw})
		if err := writer.WriteMessages(ctx, kafka.Message{Key: []byte(cmd.RequestID), Value: body}); err != nil {
			log.Printf("CONSUMER publish result for %s failed: %v (will redeliver)", cmd.RequestID, err)
			time.Sleep(retry)
			continue
		}
		_ = reader.CommitMessages(ctx, msg)
	}
}

// write is the search-first membership write against the in-memory store;
// errOutage while the switch is on.
func (s *store) write(cmd requested) (completed, error) {
	if s.down.Load() {
		return completed{}, errOutage
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	email := strings.ToLower(strings.TrimSpace(cmd.Email))
	out := completed{RequestID: cmd.RequestID, ProjectContactID: cmd.ProjectContactID}
	var c *contact
	for _, x := range s.contacts {
		if x.Email != nil && *x.Email == email {
			c = x
			break
		}
	}
	if c == nil {
		c = &contact{ID: s.nextID("003"), Email: str(email), Name: str(strings.TrimSpace(cmd.FirstName + " " + cmd.LastName)),
			FirstName: str(cmd.FirstName), LastName: str(cmd.LastName), IsCsAdmin: boolp(false), IsCsIntegrationUser: boolp(cmd.IsCsIntegrationUser),
			LockoutStatus: boolp(true), Account: &contactAccount{ID: str(cmd.AccountSfID), Classification: str("Customer"), IsPartner: boolp(false)}, LastModifiedDate: now()}
		s.contacts[c.ID] = c
		out.ContactCreated = true
	}
	out.ContactSfID = c.ID
	state := cmd.State
	if state == "" {
		state = "INVITED"
	}
	var m *projectContact
	for _, x := range s.memberships {
		if x.Subscription != nil && x.Subscription.ID != nil && *x.Subscription.ID == cmd.ProjectSfID && x.Contact != nil && x.Contact.ID != nil && *x.Contact.ID == c.ID {
			m = x
			break
		}
	}
	if m == nil {
		m = &projectContact{ID: s.nextID("a0e"), Email: email, State: str(state), Roles: cmd.Roles, Type: str("OWN CONTACT"),
			Contact:      &pcContact{ID: str(c.ID), Name: c.Name, Email: c.Email, CustomerID: str(cmd.AccountSfID)},
			Subscription: &subscription{ID: str(cmd.ProjectSfID), Name: str("Mock Project"), Key: str("MOCKPROJ"), CustomerID: str(cmd.AccountSfID)},
			CreatedDate:  now(), LastModifiedDate: now()}
		s.memberships[m.ID] = m
		out.MembershipCreated = true
	} else {
		m.State, m.Roles, m.LastModifiedDate = str(state), cmd.Roles, now()
	}
	out.MembershipSfID, out.State, out.Roles, out.LastModifiedDate = m.ID, *m.State, m.Roles, *m.LastModifiedDate
	return out, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
