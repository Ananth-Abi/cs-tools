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

// Package entityservice is a minimal HTTP client for entity-service's
// event_outbox endpoints. This service never talks to a database directly
// (see internal/handler/events.go's PostEvent) — deduplicating a retried or
// racing POST /events call is entity-service's job, via the durable claim
// this client calls into.
package entityservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
)

// EventOutboxRow is one event_outbox row, as returned by
// SearchWaitingEventOutbox. Field names and JSON tags mirror
// entity-service's domain.EventOutbox.
type EventOutboxRow struct {
	ID        string          `json:"id"`
	EventType string          `json:"eventType"`
	EntityID  string          `json:"entityId"`
	Payload   json.RawMessage `json:"payload"`
	CreatedOn time.Time       `json:"createdOn"`
}

// Client calls a single entity-service instance. entity-service has no
// inbound auth of its own (trusted at the Choreo gateway, same model as
// this service — see CLAUDE.md), so no credentials are sent here.
type Client struct {
	baseURL string
	http    *http.Client
}

// New constructs a Client against baseURL (e.g. "http://localhost:8080" or
// entity-service's Choreo URL) — no trailing slash required, one is
// stripped if present.
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// UpdateEventOutboxStatus calls entity-service's PATCH /event-outbox/{id}
// with the given target status ("dispatching", "dispatched", or "waiting" —
// see domain.UpdateEventOutboxStatusRequest on the entity-service side for
// which current status each requires).
//
// Returns *apierror.Error on any non-2xx response. Callers must check for
// StatusCode == http.StatusConflict (409) specifically — that's the expected
// outcome when another caller already claimed or dispatched the row, not a
// failure this call should be retried for.
func (c *Client) UpdateEventOutboxStatus(ctx context.Context, id, status string) error {
	body, err := json.Marshal(struct {
		Status string `json:"status"`
	}{Status: status})
	if err != nil {
		return fmt.Errorf("entityservice: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch,
		c.baseURL+"/event-outbox/"+url.PathEscape(id), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("entityservice: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("entityservice: patch event_outbox %s: %w", id, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("entityservice: read response for event_outbox %s: %w", id, err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}
	return nil
}

// SearchWaitingEventOutbox calls entity-service's POST /event-outbox/search
// for up to limit rows still in "waiting" status, oldest first — the polling
// fallback's candidate list (see outbox.Poller). No filters are sent;
// entity-service defaults an omitted status filter to "waiting" itself.
func (c *Client) SearchWaitingEventOutbox(ctx context.Context, limit int) ([]EventOutboxRow, error) {
	body, err := json.Marshal(struct {
		Pagination struct {
			Limit int `json:"limit"`
		} `json:"pagination"`
	}{Pagination: struct {
		Limit int `json:"limit"`
	}{Limit: limit}})
	if err != nil {
		return nil, fmt.Errorf("entityservice: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/event-outbox/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("entityservice: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("entityservice: search event_outbox: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("entityservice: read response for event_outbox search: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	var result struct {
		Events []EventOutboxRow `json:"events"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("entityservice: decode event_outbox search response: %w", err)
	}
	return result.Events, nil
}
