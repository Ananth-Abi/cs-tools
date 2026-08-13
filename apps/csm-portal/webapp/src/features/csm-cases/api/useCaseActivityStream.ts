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

import { useEffect } from "react";
import { useAsgardeo } from "@asgardeo/react";
import EventSourcePolyfill from "@sanity/eventsource";
import { useQueryClient } from "@tanstack/react-query";
import { apiConfig } from "@config/apiConfig";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useLogger } from "@hooks/useLogger";

/** Delay before reconnecting after the stream errors out or drops. */
const RECONNECT_DELAY_MS = 3_000;

/**
 * Opens a live Server-Sent Events connection to csm-portal-backend's
 * `GET /cases/{id}/activities/stream` (its dedicated :9092 listener — see
 * that backend's cmd/server/main.go) and invalidates the case's comments and
 * activities queries whenever it emits a `case_updated` event, so the
 * Activities tab reflects a new comment or status change without the viewer
 * having to wait out CSM_CASE_COMMENTS'/CSM_CASE_ACTIVITIES' own staleTime or
 * refresh manually.
 *
 * Uses `@sanity/eventsource` rather than the browser's native `EventSource`
 * because native EventSource cannot set custom headers — it only supports
 * cookies/query params for auth — and this stream is behind the same
 * `x-jwt-assertion`/`x-user-id-token` header auth as every other backend
 * call (see useAuthApiClient.ts). There is no separate ticket/token-exchange
 * step: the polyfill attaches those headers directly on the connection.
 *
 * Headers are fixed at construction time, so they can't be refreshed on the
 * library's own built-in reconnect — a token that expires mid-connection
 * would otherwise have the polyfill retry forever with the same stale
 * header. Instead, `error` closes the current connection and this hook
 * opens a fresh one with newly-fetched tokens after RECONNECT_DELAY_MS,
 * rather than relying on that built-in retry.
 *
 * A no-op when `caseId` is unset or `apiConfig.streamUrl` isn't configured
 * (Event Hub — and therefore this endpoint — is optional on the backend);
 * callers fall back to the comments/activities queries' own staleTime.
 */
export function useCaseActivityStream(caseId: string | undefined): void {
  const queryClient = useQueryClient();
  const { getAccessToken, getIdToken } = useAsgardeo();
  const logger = useLogger();

  useEffect(() => {
    if (!caseId || !apiConfig.streamUrl) return;

    let cancelled = false;
    let source: EventSourcePolyfill | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined;

    const connect = async (): Promise<void> => {
      let token: string | undefined;
      let idToken: string | undefined;
      try {
        [token, idToken] = await Promise.all([getAccessToken(), getIdToken()]);
      } catch (error) {
        logger.debug("[case-activity-stream] failed to get tokens", error);
      }
      if (cancelled) return;
      if (!token || !idToken) {
        reconnectTimer = setTimeout(() => void connect(), RECONNECT_DELAY_MS);
        return;
      }

      const url = `${apiConfig.streamUrl}/cases/${encodeURIComponent(caseId)}/activities/stream`;
      source = new EventSourcePolyfill(url, {
        headers: {
          Authorization: `Bearer ${token}`,
          "x-user-id-token": idToken,
        },
      });

      source.addEventListener("case_updated", () => {
        void queryClient.invalidateQueries({
          queryKey: [ApiQueryKeys.CSM_CASE_COMMENTS, caseId],
        });
        void queryClient.invalidateQueries({
          queryKey: [ApiQueryKeys.CSM_CASE_ACTIVITIES, caseId],
        });
      });

      source.addEventListener("error", () => {
        logger.debug("[case-activity-stream] connection error, reconnecting", { caseId });
        source?.close();
        if (!cancelled) {
          reconnectTimer = setTimeout(() => void connect(), RECONNECT_DELAY_MS);
        }
      });
    };

    void connect();

    return () => {
      cancelled = true;
      clearTimeout(reconnectTimer);
      source?.close();
    };
  }, [caseId, queryClient, getAccessToken, getIdToken, logger]);
}
