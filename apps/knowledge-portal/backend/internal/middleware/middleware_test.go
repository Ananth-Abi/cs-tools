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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoveryReturns500Envelope(t *testing.T) {
	h := Logger(Recovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom with internal detail")
	})))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "internal detail") || !strings.Contains(rec.Body.String(), `"message"`) {
		t.Errorf("body = %s", rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
}

func TestRecoveryRepanicsAbortHandler(t *testing.T) {
	defer func() {
		if r := recover(); r != http.ErrAbortHandler {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", r)
		}
	}()
	Recovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
}

func TestCorrelationID(t *testing.T) {
	tests := []struct {
		name     string
		inbound  string
		wantKept bool
	}{
		{"absent", "", false},
		{"well-formed kept", "req-123_abc.4", true},
		{"uuid kept", "11111111-1111-4111-8111-111111111111", true},
		{"too long replaced", strings.Repeat("a", 65), false},
		{"space replaced", "a b", false},
		{"crlf replaced", "a\r\nX-Evil: 1", false},
		{"slash replaced", "a/b", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctxID string
			h := CorrelationID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				ctxID = CorrelationIDFromContext(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if tt.inbound != "" {
				req.Header.Set(correlationIDHeader, tt.inbound)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			got := rec.Header().Get(correlationIDHeader)
			if got == "" || got != ctxID {
				t.Fatalf("header %q, context %q", got, ctxID)
			}
			if tt.wantKept != (got == tt.inbound) {
				t.Errorf("inbound %q, echoed %q, wantKept %v", tt.inbound, got, tt.wantKept)
			}
			if !validCorrelationID(got) {
				t.Errorf("echoed id %q is not itself valid", got)
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	SecurityHeaders(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff missing")
	}
}
