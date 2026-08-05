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

package entity

import (
	"net/http"
	"testing"
)

func TestNewUpstreamError_ExtractsMessageField(t *testing.T) {
	raw := []byte(`{"code":400,"message":"caseTypes must be valid UUIDs"}`)

	err := newUpstreamError(http.StatusBadRequest, raw)

	if err.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", err.StatusCode)
	}
	if err.Body != "caseTypes must be valid UUIDs" {
		t.Fatalf("expected extracted message, got %q", err.Body)
	}
}

func TestNewUpstreamError_FallsBackToRawBodyWhenNotJSON(t *testing.T) {
	raw := []byte("<html>502 Bad Gateway</html>")

	err := newUpstreamError(http.StatusBadGateway, raw)

	if err.Body != string(raw) {
		t.Fatalf("expected raw body fallback, got %q", err.Body)
	}
}

func TestNewUpstreamError_TruncatesLongRawBody(t *testing.T) {
	raw := make([]byte, maxErrBodyBytes+100)
	for i := range raw {
		raw[i] = 'x'
	}

	err := newUpstreamError(http.StatusInternalServerError, raw)

	if len(err.Body) != maxErrBodyBytes {
		t.Fatalf("expected body truncated to %d bytes, got %d", maxErrBodyBytes, len(err.Body))
	}
}
