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

import { useCallback } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { apiConfig } from "@config/apiConfig";
import {
  ASGARDEO_UNAUTHENTICATED_CODE,
  AUTH_NOT_READY_ERROR_MESSAGE,
} from "@constants/apiConstants";
import { useLogger } from "@hooks/useLogger";
import { CORRELATION_ID_HEADER, newCorrelationId } from "@utils/correlationId";

// Shared across every caller's hook instance. Each useAuthApiClient() call
// creates its own authFetch closure, so this lives at module scope to ensure
// only ONE full sign-in redirect is triggered even when many concurrent calls
// fail authentication at once.
let signInInFlight = false;

// Shared across every caller's hook instance for the same reason as
// `signInInFlight`: many concurrent requests can discover a dead refresh
// token at once, and they should all await the SAME hidden-iframe silent
// sign-in attempt rather than each opening their own.
let silentSignInInFlight: Promise<boolean> | null = null;

// Only the Asgardeo "unauthenticated" code means the token was expired/missing
// when the call ran (e.g. the refresh token itself has expired, so the SDK's
// periodic background refresh can no longer mint a new access token). Anything
// else (network failures, real backend 5xx) must propagate untouched so
// existing error handling and error pages still work. Without this
// classification, a dead refresh token sends the SDK's periodic background
// refresh into an infinite loop of failing refresh-grant requests instead of
// bouncing the user to sign-in.
function isTokenExpiredError(error: unknown): boolean {
  return (
    error != null &&
    typeof error === "object" &&
    "code" in error &&
    (error as { code: string }).code === ASGARDEO_UNAUTHENTICATED_CODE
  );
}

/**
 * True when `getAccessToken()` failed because the Asgardeo SDK had not finished
 * initializing yet (code `SPA-AUTH_CLIENT-VM-NF01`, "The SDK must be
 * initialized first"). This is a transient race on first paint — the silent
 * refresh added in @asgardeo/react 0.25.5 can ask for a token a tick before the
 * SDK is ready — so callers should treat it as "auth not ready, retry", not a
 * hard error.
 */
function isSdkNotInitializedError(error: unknown): boolean {
  if (!(error instanceof Error)) return false;
  if ((error as { code?: string }).code === "SPA-AUTH_CLIENT-VM-NF01") {
    return true;
  }
  return /SDK (?:must be initialized|is not initialized)/i.test(
    `${error.name} ${error.message}`,
  );
}

// sessionStorage marker used to stop the chain below from redirect-looping
// forever against an account whose 401 genuinely can't be fixed by
// re-authenticating (e.g. a valid token but a backend/upstream-data problem
// on that account). A short window is enough to recognise "we just forced a
// sign-in and immediately got the SAME 401 again" without misfiring on a
// user who legitimately signs in again minutes later for an unrelated
// reason — see `recentlyForcedSignIn`/`markForcedSignIn` below.
const FORCED_SIGN_IN_GUARD_KEY = "csm.auth.lastForcedSignInAt";
const FORCED_SIGN_IN_GUARD_WINDOW_MS = 10_000;

function recentlyForcedSignIn(): boolean {
  try {
    const raw = sessionStorage.getItem(FORCED_SIGN_IN_GUARD_KEY);
    if (!raw) return false;
    const lastForcedAt = Number(raw);
    return (
      Number.isFinite(lastForcedAt) &&
      Date.now() - lastForcedAt < FORCED_SIGN_IN_GUARD_WINDOW_MS
    );
  } catch {
    // sessionStorage can throw (private browsing, storage disabled) — fail
    // open so this never blocks a legitimate redirect.
    return false;
  }
}

function markForcedSignIn(): void {
  try {
    sessionStorage.setItem(FORCED_SIGN_IN_GUARD_KEY, String(Date.now()));
  } catch {
    // Best-effort only; if we can't record it, the guard simply can't help
    // this time — the redirect below still proceeds.
  }
}

// One attempt's outcome, classified so the retry chain below can treat "the
// SDK threw because the token is expired/missing" and "the fetch resolved
// with a genuine server-side HTTP 401" identically: both are `recoverable`
// (retry, then silent sign-in, then a full sign-in redirect might fix them).
// Anything else — a successful response, a non-401 error response, or a
// thrown error that isn't a token-expiry — is NOT recoverable here and is
// returned/rethrown to the caller untouched.
type AttemptOutcome =
  | { recoverable: false; response: Response }
  | { recoverable: true; response?: Response; error?: unknown };

async function runAttempt(
  attemptFetch: (input: RequestInfo | URL, options?: RequestInit) => Promise<Response>,
  input: RequestInfo | URL,
  options?: RequestInit,
): Promise<AttemptOutcome> {
  try {
    const response = await attemptFetch(input, options);
    if (response.status === 401) {
      return { recoverable: true, response };
    }
    return { recoverable: false, response };
  } catch (error) {
    if (isTokenExpiredError(error)) {
      return { recoverable: true, error };
    }
    // Non-auth thrown error (network, auth-not-ready, etc.) must surface to
    // existing error handling untouched.
    throw error;
  }
}

// Origin we are willing to attach the bearer token to. Computed once at module
// load so we don't accidentally send credentials anywhere else.
const trustedBackendOrigin = (() => {
  try {
    return new URL(apiConfig.backendUrl).origin;
  } catch {
    return "";
  }
})();

function resolveRequestUrl(input: RequestInfo | URL): URL {
  if (input instanceof Request) return new URL(input.url, window.location.origin);
  if (input instanceof URL) return input;
  return new URL(input.toString(), window.location.origin);
}

function buildRequestHeaders(
  input: RequestInfo | URL,
  options: RequestInit | undefined,
  token: string,
  idToken: string,
  correlationId: string,
): Headers {
  // When `input` is a Request, `init.headers` on the outer fetch call REPLACES
  // the request's headers wholesale — it does not merge. Seed the headers from
  // the Request and let any explicit option-level headers override.
  const headers =
    input instanceof Request ? new Headers(input.headers) : new Headers();
  if (options?.headers) {
    new Headers(options.headers).forEach((value, key) => {
      headers.set(key, value);
    });
  }

  headers.set("Authorization", `Bearer ${token}`);
  // The ID token travels alongside the access token (same convention as the
  // customer portal): the gateway validates the bearer, while the backend
  // reads the user's identity claims from `x-user-id-token`.
  headers.set("x-user-id-token", idToken);
  // Correlation ID for end-to-end tracing. The backend honours an inbound value
  // and only generates its own when absent, so a caller-supplied header (rare:
  // a retry that wants to reuse an ID) is preserved; otherwise we stamp a fresh
  // per-request UUID.
  if (!headers.has(CORRELATION_ID_HEADER)) {
    headers.set(CORRELATION_ID_HEADER, correlationId);
  }
  if (!headers.has("Accept")) {
    headers.set("Accept", "application/json");
  }

  // Inherit method/body from the Request when callers omit them in `options`.
  const method =
    options?.method?.toUpperCase() ||
    (input instanceof Request ? input.method.toUpperCase() : "GET");
  const body =
    options?.body ?? (input instanceof Request ? input.body : undefined);

  if (["POST", "PUT", "PATCH"].includes(method) && body) {
    const isNonJsonType =
      body instanceof FormData ||
      body instanceof Blob ||
      body instanceof ArrayBuffer ||
      (typeof URLSearchParams !== "undefined" && body instanceof URLSearchParams) ||
      (typeof ReadableStream !== "undefined" && body instanceof ReadableStream) ||
      ArrayBuffer.isView(body);

    if (!isNonJsonType && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
  }

  return headers;
}

// Fetch wrapper that attaches a fresh IdP access token as the bearer and the
// ID token as `x-user-id-token` (the customer portal's convention). The
// Choreo gateway validates the access token and forwards it upstream as
// `x-jwt-assertion`, which csm-portal-backend reads in its auth middleware;
// `x-user-id-token` passes through to the backend untouched.
// The tokens are only attached when the request origin matches the configured
// backend; calls to any other origin are refused so credentials can't be
// leaked to third-party hosts.
export function useAuthApiClient() {
  const { getAccessToken, getIdToken, signIn, signInSilently } = useAsgardeo();
  const logger = useLogger();

  // Redirect to a full sign-in, single-flighted so concurrent auth failures
  // don't fire multiple redirects. Returns a never-resolving promise so
  // callers don't fall through to an error page while the browser navigates
  // away.
  const redirectToSignIn = useCallback((): Promise<Response> => {
    if (!signInInFlight) {
      signInInFlight = true;
      void Promise.resolve(signIn()).finally(() => {
        signInInFlight = false;
      });
    }
    return new Promise<Response>(() => {});
  }, [signIn]);

  // Before giving up and bouncing the whole tab to a full sign-in redirect
  // (which discards any in-progress work — an open comment draft, an unsaved
  // dialog), try a silent, hidden-iframe re-authentication. If the user's IdP
  // session (SSO cookie) is still alive, this mints a fresh token without any
  // visible navigation; only a genuinely dead IdP session falls through to
  // `redirectToSignIn`. Single-flighted for the same reason as sign-in above.
  const trySilentSignIn = useCallback((): Promise<boolean> => {
    if (!silentSignInInFlight) {
      silentSignInInFlight = Promise.resolve(signInSilently())
        .then((result) => Boolean(result))
        .catch((error) => {
          logger.debug("[auth] silent sign-in failed", error);
          return false;
        })
        .finally(() => {
          silentSignInInFlight = null;
        });
    }
    return silentSignInInFlight;
  }, [signInSilently, logger]);

  const attemptFetch = useCallback(
    async (input: RequestInfo | URL, options?: RequestInit): Promise<Response> => {
      const url = resolveRequestUrl(input);
      if (!trustedBackendOrigin || url.origin !== trustedBackendOrigin) {
        throw new Error(
          `Refusing to send access token to untrusted origin ${url.origin}`,
        );
      }

      let token: string | undefined;
      let idToken: string | undefined;
      try {
        [token, idToken] = await Promise.all([
          getAccessToken(),
          getIdToken(),
        ]);
      } catch (error) {
        // Normalise the SDK-not-initialized race into the shared "auth not
        // ready" signal so callers warn-and-retry instead of surfacing a raw
        // AsgardeoAuthException as a hard error.
        if (isSdkNotInitializedError(error)) {
          throw new Error(AUTH_NOT_READY_ERROR_MESSAGE);
        }
        throw error;
      }
      if (!token) {
        throw new Error("Unable to retrieve access token");
      }
      if (!idToken) {
        throw new Error("Unable to retrieve ID token");
      }

      // One correlation ID per physical request (React Query retries each get a
      // distinct one, matching the backend's per-request unit). A caller that
      // pre-set the header keeps its value; we log whichever ID actually ships.
      const headers = buildRequestHeaders(
        input,
        options,
        token,
        idToken,
        newCorrelationId(),
      );
      const correlationId = headers.get(CORRELATION_ID_HEADER) ?? "";
      const method = (
        options?.method ??
        (input instanceof Request ? input.method : "GET")
      ).toUpperCase();

      // Centralised FE access log, mirroring the backend's request-logging
      // middleware: every backend call is logged once here with the same
      // correlation ID that backend + entity-service stamp on their log lines.
      try {
        const response = await fetch(input, { ...options, headers });
        const line = `[api] ${method} ${url.pathname} -> ${response.status} correlationID=${correlationId}`;
        if (response.ok) {
          logger.debug(line);
        } else {
          logger.error(line);
        }
        return response;
      } catch (error) {
        logger.error(
          `[api] ${method} ${url.pathname} -> network error correlationID=${correlationId}`,
          error,
        );
        throw error;
      }
    },
    [getAccessToken, getIdToken, logger],
  );

  return useCallback(
    async (input: RequestInfo | URL, options?: RequestInit): Promise<Response> => {
      // Attempt 1 of 3. Only a recoverable outcome (thrown token-expiry, or a
      // resolved genuine HTTP 401) continues the chain below; a success, a
      // non-401 error response, or a non-auth thrown error return/throw here.
      const first = await runAttempt(attemptFetch, input, options);
      if (!first.recoverable) return first.response;

      // A concurrent caller, or the provider's periodic background refresh,
      // may have re-minted the token in the meantime, so retry once to pick
      // it up. If nothing refreshed it the retry fails again and we fall
      // through to the sign-in path below.
      const retry = await runAttempt(attemptFetch, input, options);
      if (!retry.recoverable) return retry.response;
      let last = retry;

      // Still unauthenticated (or still a genuine 401) after the retry.
      // Try a silent re-auth first: if the IdP session is still alive this
      // mints a fresh token with no visible navigation, so in-progress work
      // survives.
      if (await trySilentSignIn()) {
        const afterSilentSignIn = await runAttempt(attemptFetch, input, options);
        if (!afterSilentSignIn.recoverable) return afterSilentSignIn.response;
        last = afterSilentSignIn;
        // Silent sign-in reported success but the token still won't
        // authenticate (e.g. a race with a session that expired a moment
        // later, or a genuine 401 that re-auth can't fix at all) — fall
        // through to the hard redirect below.
      }

      // Attempt 3 of 3 exhausted with no recovery. Before bouncing the whole
      // tab to a full sign-in redirect, guard against redirect-looping
      // forever against an account whose 401 isn't actually fixable by
      // re-authenticating (a valid token, but e.g. a backend/upstream-data
      // problem on that account) — if we already forced a sign-in very
      // recently and landed right back on the same failure, let it surface
      // as a normal failure instead of redirecting again.
      if (recentlyForcedSignIn()) {
        if (last.response) return last.response;
        throw last.error;
      }
      markForcedSignIn();

      // Silent re-auth was unavailable, or the IdP session itself is gone,
      // or re-auth succeeded but the same 401 persists. Redirect for a full
      // sign-in instead of letting the SDK's periodic refresh (or this
      // caller) keep retrying forever.
      return redirectToSignIn();
    },
    [attemptFetch, redirectToSignIn, trySilentSignIn],
  );
}
