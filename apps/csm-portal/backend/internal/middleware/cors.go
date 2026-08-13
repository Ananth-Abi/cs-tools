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

package middleware

import "net/http"

// corsAllowedHeaders lists every request header the browser-based
// case-activity SSE client (@sanity/eventsource's XHR-based browser
// polyfill — unlike a WebSocket handshake, a plain XHR/fetch call to a
// cross-origin listener IS subject to CORS) may need to send. Authorization
// is what the frontend actually sets (see useAuthApiClient.ts /
// useCaseActivityStream.ts) — Choreo's gateway is what translates it into
// x-jwt-assertion before this backend ever sees it in a real deployment
// (confirmed: Choreo's own CORS response for :8080 already allow-lists
// "authorization", not "x-jwt-assertion") — but this listener allow-lists
// x-jwt-assertion too, both for local testing that bypasses the gateway
// (see the matching fallback in Auth) and as defense-in-depth.
const corsAllowedHeaders = "Authorization, x-jwt-assertion, x-user-id-token, X-CSM-Correlation-ID"

// CORS returns an HTTP middleware handling cross-origin requests to the
// case-activity SSE listener (:9092 — see cmd/server/main.go). Only that
// listener needs this: the main :8080 REST API is fronted by Choreo's API
// gateway in every real deployment, which adds CORS headers itself, but a
// second, freshly-added Choreo endpoint isn't guaranteed to inherit that
// automatically, and local dev bypasses the gateway entirely — so this
// backend handles it directly for :9092 rather than assuming either.
//
// MUST be the outermost middleware in that listener's chain (wrapping Auth,
// not wrapped by it): a CORS preflight is an OPTIONS request with no
// x-jwt-assertion header at all, so if Auth ran first it would reject every
// preflight with 401 before the browser ever saw a CORS header — which the
// browser reports as "blocked by CORS policy", masking the real cause. See
// apps/customer-portal/backend-v2's identically-named middleware, whose
// doc comment this mirrors.
//
// allowedOrigins is an allow-list of browser Origins; an empty list allows
// any origin, matching this backend's local-development-friendly default —
// the same tradeoff apps/customer-portal/backend-v2's CORS makes, safe for
// the same reason: this backend authenticates via a caller-supplied
// x-jwt-assertion header, never cookies, so there is no session credential
// for a browser to attach automatically, and Access-Control-Allow-Credentials
// is deliberately never set below. If this backend ever adds cookie-based
// auth, allowedOrigins MUST become a real non-empty allow-list first — an
// unrestricted origin plus credentials lets any site read authenticated
// responses on the victim's behalf.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (len(allowed) == 0 || allowed[origin]) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
				w.Header().Set("Access-Control-Max-Age", "3600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
