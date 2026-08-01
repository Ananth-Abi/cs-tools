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

package aichatagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/oauth2/clientcredentials"
)

// eventTypeKey/eventPayloadKey/eventFinal/eventError mirror the upstream AI
// chat agent's WebSocket event envelope — see apps/customer-portal/backend's
// modules/ai_chat_agent/constants.bal.
const (
	eventTypeKey    = "type"
	eventPayloadKey = "payload"
	eventFinal      = "final"
	eventError      = "error"
)

// WSConfig holds the configuration for dialing the upstream AI chat agent's
// WebSocket endpoint. Kept separate from Config since the Ballerina backend
// this is rewriting uses a distinct OAuth2 client-credentials configuration
// for its WebSocket connection.
type WSConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// WSClient dials the upstream AI chat agent's WebSocket endpoint.
type WSClient struct {
	baseURL string
	oauth   *clientcredentials.Config
}

// NewWSClient constructs a WSClient authenticated via the OAuth2 client
// credentials grant.
func NewWSClient(cfg WSConfig) *WSClient {
	return &WSClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		oauth: &clientcredentials.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			TokenURL:     cfg.TokenURL,
			Scopes:       cfg.Scopes,
		},
	}
}

// dial opens a WebSocket connection to the upstream AI chat agent for the
// given session ID, authenticated with a bearer token obtained via OAuth2
// client credentials.
func (c *WSClient) dial(ctx context.Context, sessionID string) (*websocket.Conn, error) {
	token, err := c.oauth.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("aichatagent: fetch WS token: %w", err)
	}

	wsURL := strings.Replace(c.baseURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL += "/ws?sessionId=" + url.QueryEscape(sessionID)

	header := http.Header{}
	header.Set("Authorization", "Bearer "+token.AccessToken)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return nil, fmt.Errorf("aichatagent: dial upstream WebSocket: %w", err)
	}
	return conn, nil
}

// BrowserConn abstracts the browser-facing WebSocket connection just enough
// for StreamChat to forward events to it, so the handler package's real
// *websocket.Conn doesn't need to be imported here.
type BrowserConn interface {
	WriteMessage(messageType int, data []byte) error
}

// StreamChat opens a dedicated upstream WebSocket connection for sessionID,
// sends payload, then forwards every event verbatim to caller until a
// "final" or "error" event arrives or the upstream connection closes.
// Mirrors apps/customer-portal/backend's ai_chat_agent:streamChat.
func (c *WSClient) StreamChat(ctx context.Context, sessionID, payload string, caller BrowserConn) (map[string]json.RawMessage, error) {
	conn, err := c.dial(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(payload)); err != nil {
		return nil, fmt.Errorf("aichatagent: write initial message: %w", err)
	}

	finalPayload := map[string]json.RawMessage{}
	for {
		if dl, ok := ctx.Deadline(); ok {
			_ = conn.SetReadDeadline(dl)
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				break
			}
			errPayload, _ := json.Marshal(map[string]string{"type": eventError, "message": err.Error()}) // #nosec G104 -- best-effort forward, connection may already be gone
			_ = caller.WriteMessage(websocket.TextMessage, errPayload)
			break
		}

		if writeErr := caller.WriteMessage(websocket.TextMessage, data); writeErr != nil {
			break
		}

		var parsed map[string]json.RawMessage
		if err := json.Unmarshal(data, &parsed); err != nil {
			continue
		}
		var evtType string
		if raw, ok := parsed[eventTypeKey]; ok {
			_ = json.Unmarshal(raw, &evtType)
		}
		if evtType == eventFinal {
			if raw, ok := parsed[eventPayloadKey]; ok {
				var nested map[string]json.RawMessage
				if err := json.Unmarshal(raw, &nested); err == nil {
					finalPayload = nested
					break
				}
			}
			finalPayload = parsed
			break
		}
		if evtType == eventError {
			break
		}
	}

	_ = conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session complete"),
		time.Now().Add(2*time.Second))

	return finalPayload, nil
}
