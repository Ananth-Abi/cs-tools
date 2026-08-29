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

// Package notify sends report/alert emails directly against the same
// internal email notification service integrations/csm-notification-service
// uses (POST /send-email, OAuth2 client credentials) — this component has
// no event-emission hook yet (see this component's own CLAUDE.md, "Future:
// events"), so it sends its own email rather than publishing an event for
// csm-notification-service to relay.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// emailTokenFetchTimeout is the HTTP client timeout for token-endpoint
// requests. Overridden in tests to keep them fast.
var emailTokenFetchTimeout = 10 * time.Second

// Config holds the configuration for the email client below.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	// FromAddress is the fixed "From" address used for every outgoing
	// email — a config value, not a SendEmail argument, so every email
	// this component sends comes from one pre-approved sender.
	FromAddress string
}

// Client is an HTTP client for the internal email notification service,
// authenticated via the OAuth2 client credentials grant. Tokens are
// acquired and refreshed automatically.
//
// NewClient never fails and never contacts the token endpoint, so it is
// safe to construct with a zero-value Config — a missing/invalid
// configuration only surfaces as an error the first time SendEmail is
// called, matching integrations/csm-notification-service's own
// internal/notifications.EmailClient.
type Client struct {
	http        *http.Client
	baseURL     string
	fromAddress string
}

// NewClient constructs a Client that authenticates against the email
// notification service using the OAuth2 client credentials grant type.
func NewClient(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: emailTokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second

	return &Client{
		http:        httpClient,
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		fromAddress: cfg.FromAddress,
	}
}

// sendEmailRequest is the wire shape expected by POST /send-email.
type sendEmailRequest struct {
	To       []string `json:"to"`
	From     string   `json:"from"`
	Subject  string   `json:"subject"`
	Template []byte   `json:"template"`
}

// SendEmail sends a plain HTML email via the notification service. The
// sender address is always the client's configured FromAddress. A request
// with no recipients is a silent no-op — see registry.Task.ReportRecipients'
// own doc comment for why that's a deliberate, valid configuration rather
// than an error.
func (c *Client) SendEmail(ctx context.Context, to []string, subject, htmlBody string) error {
	if len(to) == 0 {
		return nil
	}
	if subject == "" {
		return fmt.Errorf("notify: subject is required")
	}

	reqBody, err := json.Marshal(sendEmailRequest{
		To:       to,
		From:     c.fromAddress,
		Subject:  subject,
		Template: []byte(htmlBody),
	})
	if err != nil {
		return fmt.Errorf("notify: encode send-email request: %w", err)
	}

	var reqReader io.Reader = bytes.NewReader(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/send-email", reqReader)
	if err != nil {
		return fmt.Errorf("notify: build send-email request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("notify: send-email: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("notify: read send-email response: %w", err)
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
