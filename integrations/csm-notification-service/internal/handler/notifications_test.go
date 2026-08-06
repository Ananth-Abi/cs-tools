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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

func newTestHandler() *NotificationHandler {
	return NewNotificationHandler(
		notifications.NewEmailClient(notifications.EmailConfig{}),
		notifications.NewGoogleChatClient(notifications.GoogleChatConfig{}),
		notifications.NewTwilioClient(notifications.TwilioConfig{}),
	)
}

func postNotification(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/notifications", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.PostNotification(w, r)
	return w
}

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status = %d, want %d (body=%s)", w.Code, want, w.Body.String())
	}
}

func TestPostNotification_ValidEmail(t *testing.T) {
	w := postNotification(t, `{"channel":"email","email":{"to":["a@example.com"],"subject":"hi"}}`)
	assertStatus(t, w, http.StatusAccepted)
}

func TestPostNotification_ValidGoogleChat(t *testing.T) {
	w := postNotification(t, `{"channel":"googleChat","googleChat":{"product":"api-manager","title":"t","shortDescription":"d","caseId":"CASE-1"}}`)
	assertStatus(t, w, http.StatusAccepted)
}

func TestPostNotification_ValidSms(t *testing.T) {
	w := postNotification(t, `{"channel":"sms","sms":{"to":"+15551234567","body":"On-call page: P1 incident"}}`)
	assertStatus(t, w, http.StatusAccepted)
}

func TestPostNotification_ValidCall(t *testing.T) {
	w := postNotification(t, `{"channel":"call","call":{"to":"+15551234567","body":"On-call page: P1 incident"}}`)
	assertStatus(t, w, http.StatusAccepted)
}

func TestPostNotification_RequiresFields(t *testing.T) {
	cases := map[string]string{
		"email missing to":                    `{"channel":"email","email":{"subject":"hi"}}`,
		"email missing subject":               `{"channel":"email","email":{"to":["a@example.com"]}}`,
		"email missing payload":               `{"channel":"email"}`,
		"googleChat missing product":          `{"channel":"googleChat","googleChat":{"title":"t","shortDescription":"d","caseId":"c"}}`,
		"googleChat missing title":            `{"channel":"googleChat","googleChat":{"product":"p","shortDescription":"d","caseId":"c"}}`,
		"googleChat missing shortDescription": `{"channel":"googleChat","googleChat":{"product":"p","title":"t","caseId":"c"}}`,
		"googleChat missing caseId":           `{"channel":"googleChat","googleChat":{"product":"p","title":"t","shortDescription":"d"}}`,
		"googleChat missing payload":          `{"channel":"googleChat"}`,
		"sms missing to":                      `{"channel":"sms","sms":{"body":"hi"}}`,
		"sms missing body":                    `{"channel":"sms","sms":{"to":"+15551234567"}}`,
		"sms missing payload":                 `{"channel":"sms"}`,
		"call missing to":                     `{"channel":"call","call":{"body":"hi"}}`,
		"call missing body":                   `{"channel":"call","call":{"to":"+15551234567"}}`,
		"call missing payload":                `{"channel":"call"}`,
		"unknown channel":                     `{"channel":"fax"}`,
		"empty body":                          ``,
		"invalid json":                        `not json`,
		"unknown top-level field":             `{"channel":"email","email":{"to":["a@example.com"],"subject":"hi"},"extra":true}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			w := postNotification(t, body)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

func TestPostNotification_RejectsMismatchedPayload(t *testing.T) {
	cases := map[string]string{
		"email channel with googleChat payload also present": `{"channel":"email","email":{"to":["a@example.com"],"subject":"hi"},"googleChat":{"product":"p","title":"t","shortDescription":"d","caseId":"c"}}`,
		"email channel with sms payload also present":        `{"channel":"email","email":{"to":["a@example.com"],"subject":"hi"},"sms":{"to":"+15551234567","body":"hi"}}`,
		"email channel with call payload also present":       `{"channel":"email","email":{"to":["a@example.com"],"subject":"hi"},"call":{"to":"+15551234567","body":"hi"}}`,
		"googleChat channel with email payload also present": `{"channel":"googleChat","googleChat":{"product":"p","title":"t","shortDescription":"d","caseId":"c"},"email":{"to":["a@example.com"],"subject":"hi"}}`,
		"googleChat channel with sms payload also present":   `{"channel":"googleChat","googleChat":{"product":"p","title":"t","shortDescription":"d","caseId":"c"},"sms":{"to":"+15551234567","body":"hi"}}`,
		"googleChat channel with call payload also present":  `{"channel":"googleChat","googleChat":{"product":"p","title":"t","shortDescription":"d","caseId":"c"},"call":{"to":"+15551234567","body":"hi"}}`,
		"sms channel with email payload also present":        `{"channel":"sms","sms":{"to":"+15551234567","body":"hi"},"email":{"to":["a@example.com"],"subject":"hi"}}`,
		"sms channel with googleChat payload also present":   `{"channel":"sms","sms":{"to":"+15551234567","body":"hi"},"googleChat":{"product":"p","title":"t","shortDescription":"d","caseId":"c"}}`,
		"sms channel with call payload also present":         `{"channel":"sms","sms":{"to":"+15551234567","body":"hi"},"call":{"to":"+15551234567","body":"hi"}}`,
		"call channel with email payload also present":       `{"channel":"call","call":{"to":"+15551234567","body":"hi"},"email":{"to":["a@example.com"],"subject":"hi"}}`,
		"call channel with googleChat payload also present":  `{"channel":"call","call":{"to":"+15551234567","body":"hi"},"googleChat":{"product":"p","title":"t","shortDescription":"d","caseId":"c"}}`,
		"call channel with sms payload also present":         `{"channel":"call","call":{"to":"+15551234567","body":"hi"},"sms":{"to":"+15551234567","body":"hi"}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			w := postNotification(t, body)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

func TestPostNotification_RejectsTrailingData(t *testing.T) {
	cases := map[string]string{
		"trailing json object": `{"channel":"email","email":{"to":["a@example.com"],"subject":"hi"}}{"garbage":true}`,
		"trailing garbage":     `{"channel":"email","email":{"to":["a@example.com"],"subject":"hi"}} garbage`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			w := postNotification(t, body)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

func TestPostNotification_RejectsOversizedBody(t *testing.T) {
	h := newTestHandler()
	huge := strings.Repeat("a", maxRequestBodyBytes+1)
	body := `{"channel":"email","email":{"to":["a@example.com"],"subject":"` + huge + `"}}`
	r := httptest.NewRequest(http.MethodPost, "/notifications", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.PostNotification(w, r)
	assertStatus(t, w, http.StatusRequestEntityTooLarge)
}

func TestPostNotification_ResponseBody(t *testing.T) {
	w := postNotification(t, `{"channel":"email","email":{"to":["a@example.com"],"subject":"hi"}}`)
	var got map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if got["message"] == "" {
		t.Error("expected a non-empty message field")
	}
}
