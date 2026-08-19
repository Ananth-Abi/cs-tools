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

import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// The token-recovery chain (retry, then silent sign-in) now lives entirely
// in useAuthTokens, exercised here for real (not mocked) since this file
// only mocks @asgardeo/react's own primitives. Fake timers are advanced past
// this to let its internal promise chain settle between steps.
const POLL_INTERVAL_MS = 700;

const ASGARDEO_UNAUTHENTICATED_CODE = "SPA-AUTH_CLIENT-VM-IV02";

const getAccessTokenMock = vi.fn();
const getIdTokenMock = vi.fn();
const signInMock = vi.fn();
const signInSilentlyMock = vi.fn();

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getAccessToken: getAccessTokenMock,
    getIdToken: getIdTokenMock,
    signIn: signInMock,
    signInSilently: signInSilentlyMock,
  }),
}));

vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

const debugMock = vi.fn();
const errorMock = vi.fn();
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: debugMock,
    info: vi.fn(),
    warn: vi.fn(),
    error: errorMock,
  }),
}));

import { useAuthApiClient } from "@hooks/useAuthApiClient";

const TOKEN_EXPIRED_ERROR = { code: ASGARDEO_UNAUTHENTICATED_CODE, message: "unauthenticated" };

function jsonResponse(status: number, body: unknown = {}): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("useAuthApiClient", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal("fetch", fetchMock);
    fetchMock.mockReset();
    getAccessTokenMock.mockReset().mockResolvedValue("access-token");
    getIdTokenMock.mockReset().mockResolvedValue("id-token");
    signInMock.mockReset().mockResolvedValue(undefined);
    signInSilentlyMock.mockReset().mockResolvedValue(true);
    debugMock.mockReset();
    errorMock.mockReset();
    sessionStorage.clear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns the response untouched on a plain success", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { ok: true }));

    const { result } = renderHook(() => useAuthApiClient());
    const response = await result.current("https://example.test/api/thing");

    expect(response.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(signInSilentlyMock).not.toHaveBeenCalled();
    expect(signInMock).not.toHaveBeenCalled();
  });

  it("propagates a non-401 error response (e.g. 500) untouched, without retrying or signing in", async () => {
    fetchMock.mockResolvedValue(jsonResponse(500, { error: "boom" }));

    const { result } = renderHook(() => useAuthApiClient());
    const response = await result.current("https://example.test/api/thing");

    expect(response.status).toBe(500);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(signInSilentlyMock).not.toHaveBeenCalled();
    expect(signInMock).not.toHaveBeenCalled();
  });

  it("propagates a network error untouched, without retrying or signing in", async () => {
    fetchMock.mockRejectedValue(new Error("network down"));

    const { result } = renderHook(() => useAuthApiClient());
    await expect(result.current("https://example.test/api/thing")).rejects.toThrow(
      "network down",
    );

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(signInSilentlyMock).not.toHaveBeenCalled();
    expect(signInMock).not.toHaveBeenCalled();
  });

  it("recovers from a thrown token-expiry error via the bare retry alone, once the token is fresh", async () => {
    getAccessTokenMock
      .mockRejectedValueOnce(TOKEN_EXPIRED_ERROR)
      .mockResolvedValue("fresh-token");
    fetchMock.mockResolvedValue(jsonResponse(200, { ok: true }));

    const { result } = renderHook(() => useAuthApiClient());
    const response = await result.current("https://example.test/api/thing");

    expect(response.status).toBe(200);
    expect(signInSilentlyMock).not.toHaveBeenCalled();
    expect(signInMock).not.toHaveBeenCalled();
  });

  it("on a thrown token-expiry error that survives the bare retry, tries silent sign-in then recovers via the first poll", async () => {
    getAccessTokenMock
      .mockRejectedValueOnce(TOKEN_EXPIRED_ERROR)
      .mockRejectedValueOnce(TOKEN_EXPIRED_ERROR)
      .mockResolvedValue("fresh-token");
    fetchMock.mockResolvedValue(jsonResponse(200, { ok: true }));

    const { result } = renderHook(() => useAuthApiClient());
    const responsePromise = result.current("https://example.test/api/thing");
    // Silent sign-in's own return value isn't trusted as the sole recovery
    // signal (see useAuthApiClient.ts) — the chain polls the original
    // request on an interval instead; advance past the first one.
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
    const response = await responsePromise;

    expect(response.status).toBe(200);
    expect(signInSilentlyMock).toHaveBeenCalledTimes(1);
    expect(signInMock).not.toHaveBeenCalled();
  });

  it("forces a full sign-in redirect when silent sign-in itself reports failure, without waiting out the full poll budget", async () => {
    getAccessTokenMock.mockRejectedValue(TOKEN_EXPIRED_ERROR);
    signInSilentlyMock.mockResolvedValue(false);

    const { result } = renderHook(() => useAuthApiClient());
    void result.current("https://example.test/api/thing");

    // Silent sign-in reporting failure ends polling after the next interval,
    // not the full budget.
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
    await vi.waitFor(() => {
      expect(signInSilentlyMock).toHaveBeenCalledTimes(1);
      expect(signInMock).toHaveBeenCalledTimes(1);
    });
  });

  it("recovers a POST Request with a body from a thrown token-expiry error via the bare retry, without the body being consumed", async () => {
    getAccessTokenMock
      .mockRejectedValueOnce(TOKEN_EXPIRED_ERROR)
      .mockResolvedValue("fresh-token");
    const body = JSON.stringify({ hello: "world" });
    const requestBodies: string[] = [];
    fetchMock.mockImplementation(async (input) => {
      requestBodies.push(await (input as Request).text());
      return jsonResponse(200, { ok: true });
    });

    const request = new Request("https://example.test/api/thing", {
      method: "POST",
      body,
    });

    const { result } = renderHook(() => useAuthApiClient());
    const response = await result.current(request);

    expect(response.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(requestBodies).toEqual([body]);
    // The original Request must stay pristine (not directly fetched) so it
    // remains clonable across a future attempt.
    expect(request.bodyUsed).toBe(false);
  });
});
