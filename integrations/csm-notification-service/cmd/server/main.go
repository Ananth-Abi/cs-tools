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

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/dispatch"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entityservice"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/outbox"
)

// outboxPollMinAge is how old a "waiting" event_outbox row must be before
// outbox.Poller will claim it — see Poller.MinAge's doc comment.
const outboxPollMinAge = 10 * time.Second

// outboxPollLimit caps how many waiting rows outbox.Poller fetches per tick.
const outboxPollLimit = 50

func main() {
	loadDotEnv(".env")
	middleware.ConfigureLogger()

	// Email is not yet configured for every deployment, so its config is read
	// with os.Getenv (never mustEnv) — a missing or invalid configuration only
	// surfaces as an error the first time a caller requests the email channel.
	emailClient := notifications.NewEmailClient(notifications.EmailConfig{
		BaseURL:      os.Getenv("EMAIL_BASE_URL"),
		TokenURL:     os.Getenv("EMAIL_TOKEN_URL"),
		ClientID:     os.Getenv("EMAIL_CLIENT_ID"),
		ClientSecret: os.Getenv("EMAIL_CLIENT_SECRET"),
		Scopes:       splitComma(os.Getenv("EMAIL_SCOPES")),
		FromAddress:  os.Getenv("EMAIL_FROM_ADDRESS"),
	})

	// Google Chat is likewise optional per deployment; a missing or malformed
	// value logs a warning and yields no spaces rather than failing startup.
	googleChatClient := notifications.NewGoogleChatClient(notifications.GoogleChatConfig{
		Spaces: parseGoogleChatSpaces(os.Getenv("GOOGLE_CHAT_SPACES")),
	})

	// Twilio (the call channel, used by incident.created) is likewise
	// optional per deployment; a missing config only surfaces as an error
	// the first time dispatch requests it.
	twilioClient := notifications.NewTwilioClient(notifications.TwilioConfig{
		AccountSID:          os.Getenv("TWILIO_ACCOUNT_SID"),
		AuthToken:           os.Getenv("TWILIO_AUTH_TOKEN"),
		FromNumber:          os.Getenv("TWILIO_FROM_NUMBER"),
		MessagingServiceSid: os.Getenv("TWILIO_MESSAGING_SERVICE_SID"),
		Voice:               os.Getenv("TWILIO_VOICE"),
		Language:            os.Getenv("TWILIO_LANGUAGE"),
		APIBaseURL:          os.Getenv("TWILIO_API_BASE_URL"),
	})

	// The event bus (Azure Event Hub's Kafka-compatible endpoint) is this
	// service's core purpose, unlike the notification channels above, so its
	// config is required (mustEnv) — a misconfigured deployment should fail
	// loudly at startup instead of silently accepting or dropping events.
	eventBusCfg := eventbus.Config{
		Broker:           mustEnv("EVENT_HUB_BROKER"),
		ConnectionString: mustEnv("EVENT_HUB_CONNECTION_STRING"),
		Topic:            mustEnv("EVENT_HUB_TOPIC"),
	}
	producer := eventbus.NewProducer(eventBusCfg)
	defer producer.Close()

	consumerGroup := envOrDefault("EVENT_HUB_CONSUMER_GROUP", "csm-notification-service")
	consumer := eventbus.NewConsumer(eventBusCfg, consumerGroup)
	defer consumer.Close()

	// entity-service is optional per deployment — when ENTITY_SERVICE_BASE_URL
	// is unset, outbox claim/dispatch/release (and the poller below) is
	// skipped entirely (see handler.NewEventsHandler's doc comment).
	// Branching here rather than handing a possibly-nil *entityservice.Client
	// straight to NewEventsHandler avoids wrapping a nil pointer in a
	// non-nil interface value, which would make h.entityService != nil true
	// even when unset.
	var eventsHandler *handler.EventsHandler
	var outboxPoller *outbox.Poller
	if baseURL := os.Getenv("ENTITY_SERVICE_BASE_URL"); baseURL != "" {
		entityClient := entityservice.New(baseURL)
		eventsHandler = handler.NewEventsHandler(producer, entityClient)
		outboxPoller = &outbox.Poller{
			Publisher:     producer,
			EntityService: entityClient,
			Interval:      envDuration("EVENT_OUTBOX_POLL_INTERVAL", 30*time.Second),
			MinAge:        outboxPollMinAge,
			Limit:         outboxPollLimit,
		}
	} else {
		eventsHandler = handler.NewEventsHandler(producer, nil)
	}

	dispatcher := dispatch.NewDispatcher(emailClient, googleChatClient, twilioClient)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /events", eventsHandler.PostEvent)

	addr := ":" + mustPort("PORT", "8080")

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to bind", "addr", addr, "err", err)
		os.Exit(1)
	}
	slog.Info("CSM Notification Service started", "addr", addr)

	// No Auth layer in this middleware chain — inbound requests are trusted at the
	// Choreo API Manager gateway (subscription + M2M app auth), not validated again
	// in this service.
	srv := &http.Server{
		Handler: middleware.SecurityHeaders(
			middleware.CorrelationID(
				middleware.Logger(mux),
			),
		),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()

	// consumer.Run blocks polling for events until ctx is canceled (the same
	// shutdown signal the HTTP server responds to), so both stop together.
	go consumer.Run(ctx, dispatcher.Handle)
	slog.Info("event bus consumer started", "topic", eventBusCfg.Topic, "consumerGroup", consumerGroup)

	if outboxPoller != nil {
		go outboxPoller.Run(ctx)
		slog.Info("event_outbox poller started", "interval", outboxPoller.Interval, "minAge", outboxPoller.MinAge)
	}

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("CSM Notification Service stopped")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envDuration returns the given environment variable parsed as a
// time.Duration (e.g. "30s"), or def if unset or malformed.
func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("environment variable is not a valid duration; using default", "key", key, "value", v, "default", def)
		return def
	}
	return d
}

// mustPort returns the value of the given environment variable (or def if
// unset) as a bare port number, e.g. "8080" — not an address like ":8080" or
// "localhost:8080". Exits the process if the value isn't a valid TCP port.
func mustPort(key, def string) string {
	v := envOrDefault(key, def)
	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		slog.Error("environment variable must be a plain port number (e.g. \"8080\"), not an address", "key", key, "value", v)
		os.Exit(1)
	}
	return v
}

// loadDotEnv reads a .env file and sets any unset environment variables from it.
// Silently ignored if the file does not exist; logs a warning for any other error.
func loadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- path is always the hardcoded literal ".env" at the only call site
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("loadDotEnv: failed to open .env file", "err", err)
		}
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		// Strip surrounding quotes from value.
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if _, present := os.LookupEnv(k); !present {
			_ = os.Setenv(k, v)
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("loadDotEnv: error reading .env file", "err", err)
	}
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}

// parseGoogleChatSpaces decodes GOOGLE_CHAT_SPACES, a JSON array of
// {"product":"...","webhookUrl":"..."} objects — one per Google Chat space.
// A missing or malformed value logs a warning and yields no spaces rather
// than failing startup, since this channel is not required for every
// deployment.
func parseGoogleChatSpaces(raw string) []notifications.GoogleChatSpace {
	if raw == "" {
		return nil
	}
	var spaces []notifications.GoogleChatSpace
	if err := json.Unmarshal([]byte(raw), &spaces); err != nil {
		slog.Error("failed to parse GOOGLE_CHAT_SPACES; Google Chat alerts will be unavailable", "err", err)
		return nil
	}
	return spaces
}
