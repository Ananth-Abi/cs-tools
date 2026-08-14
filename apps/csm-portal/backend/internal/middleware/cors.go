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

// corsAllowedHeaders lists every request header a browser client may need to
// send to either listener. Authorization is what the frontend actually sets
// (see useAuthApiClient.ts / useCaseActivityStream.ts) — Choreo's gateway is
// what translates it into x-jwt-assertion before this backend ever sees it in
// a real deployment (confirmed: Choreo's own CORS response for :8080 already
// allow-lists "authorization", not "x-jwt-assertion") — but x-jwt-assertion is
// allow-listed too, both for local testing that bypasses the gateway and as
// defense-in-depth. Content-Type is required for JSON request bodies:
// application/json is not a CORS-safelisted value, so a POST/PATCH preflight
// fails without it.
const corsAllowedHeaders = "Content-Type, Authorization, x-jwt-assertion, x-user-id-token, X-CSM-Correlation-ID"

// corsAllowedMethods covers both listeners: the main REST API's full verb set
// and the stream listener's GET. Advertising a method a given listener has no
// route for is harmless — the route simply 404s if actually called.
const corsAllowedMethods = "GET, POST, PATCH, DELETE, OPTIONS"

// CORS returns an HTTP middleware handling cross-origin browser requests. It
// wraps both listeners (see cmd/server/main.go). In a real deployment Choreo's
// API gateway supplies these headers itself, making this a no-op there; it
// matters when the gateway isn't in the path — local development, where the
// browser calls a listener directly — and as defense-in-depth for the
// separately-declared stream endpoint, which isn't guaranteed to inherit the
// gateway's CORS handling the same way the long-established :8080 one does.
//
// MUST be the outermost middleware in the chain (wrapping Auth, not wrapped
// by it): a CORS preflight is an OPTIONS request with no x-jwt-assertion
// header at all, so if Auth ran first it would reject every preflight with
// 401 before the browser ever saw a CORS header — which the browser reports
// as "blocked by CORS policy", masking the real cause. Worse, Auth sits
// *before* Logger in this backend's chain, so such a rejection isn't even
// logged, making it invisible server-side. See
// apps/customer-portal/backend-v2's identically-named middleware, whose doc
// comment this mirrors.
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
				w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
				w.Header().Set("Access-Control-Max-Age", "3600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
