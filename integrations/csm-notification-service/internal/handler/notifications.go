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

package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// NotificationHandler handles HTTP requests that submit a notification for
// dispatch. emailClient and googleChatClient are held here so they're ready
// to use once PostNotification actually dispatches (see its TODO) — for now
// neither is called.
type NotificationHandler struct {
	emailClient      *notifications.EmailClient
	googleChatClient *notifications.GoogleChatClient
}

// NewNotificationHandler creates a NotificationHandler backed by the given
// channel clients.
func NewNotificationHandler(emailClient *notifications.EmailClient, googleChatClient *notifications.GoogleChatClient) *NotificationHandler {
	return &NotificationHandler{
		emailClient:      emailClient,
		googleChatClient: googleChatClient,
	}
}

// emailNotificationPayload is the body of a "email" channel notification.
type emailNotificationPayload struct {
	To          []string                        `json:"to"`
	CC          []string                        `json:"cc,omitempty"`
	BCC         []string                        `json:"bcc,omitempty"`
	ReplyTo     []string                        `json:"replyTo,omitempty"`
	Subject     string                          `json:"subject"`
	HTMLBody    string                          `json:"htmlBody"`
	Attachments []notifications.EmailAttachment `json:"attachments,omitempty"`
}

// googleChatNotificationPayload is the body of a "googleChat" channel notification.
type googleChatNotificationPayload struct {
	Product          string `json:"product"`
	Title            string `json:"title"`
	ShortDescription string `json:"shortDescription"`
	CaseID           string `json:"caseId"`
}

// sendNotificationRequest is the body accepted by PostNotification. Channel
// selects which of Email/GoogleChat is populated.
type sendNotificationRequest struct {
	Channel    string                         `json:"channel"`
	Email      *emailNotificationPayload      `json:"email,omitempty"`
	GoogleChat *googleChatNotificationPayload `json:"googleChat,omitempty"`
}

// PostNotification handles POST /notifications — the entry point other
// services call to request a notification be sent.
//
// TODO: this currently only validates and accepts the request. Once the
// Kafka-based event backbone lands, this handler should publish the
// notification event to the message queue (producer) instead of a no-op,
// so a consumer can dispatch it via emailClient/googleChatClient
// asynchronously.
func (h *NotificationHandler) PostNotification(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	var req sendNotificationRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	// Decode only consumes the first JSON value in body; reject any trailing
	// value or malformed bytes rather than silently ignoring them.
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	switch req.Channel {
	case "email":
		if req.GoogleChat != nil {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		if req.Email == nil || len(req.Email.To) == 0 || strings.TrimSpace(req.Email.Subject) == "" {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
	case "googleChat":
		if req.Email != nil {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		gc := req.GoogleChat
		if gc == nil ||
			strings.TrimSpace(gc.Product) == "" ||
			strings.TrimSpace(gc.Title) == "" ||
			strings.TrimSpace(gc.ShortDescription) == "" ||
			strings.TrimSpace(gc.CaseID) == "" {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// TODO: publish req to the message queue (Kafka producer) instead of
	// just acknowledging — see the handler doc comment above.

	writeJSONValue(w, http.StatusAccepted, map[string]string{"message": "notification accepted"})
}
