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

import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const { MockEventSource, mockInstances } = vi.hoisted(() => {
  const mockInstances: InstanceType<typeof MockEventSource>[] = [];
  class MockEventSource extends EventTarget {
    url: string;
    headers?: Record<string, string>;
    closed = false;
    constructor(url: string, init?: { headers?: Record<string, string> }) {
      super();
      this.url = url;
      this.headers = init?.headers;
      mockInstances.push(this);
    }
    close() {
      this.closed = true;
    }
  }
  return { MockEventSource, mockInstances };
});

vi.mock("@sanity/eventsource", () => ({ default: MockEventSource }));

const getAccessTokenMock = vi.fn();
const getIdTokenMock = vi.fn();
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getAccessToken: getAccessTokenMock,
    getIdToken: getIdTokenMock,
  }),
}));

const { apiConfigMock } = vi.hoisted(() => ({
  apiConfigMock: {
    backendUrl: "https://example.test",
    streamUrl: "https://stream.example.test" as string | undefined,
  },
}));
vi.mock("@config/apiConfig", () => ({ apiConfig: apiConfigMock }));

const debugMock = vi.fn();
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: debugMock,
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

const { useCaseActivityStream } = await import("./useCaseActivityStream");

const invalidateQueriesMock = vi.fn();

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  queryClient.invalidateQueries = invalidateQueriesMock.mockImplementation(() =>
    Promise.resolve(),
  );
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useCaseActivityStream", () => {
  beforeEach(() => {
    mockInstances.length = 0;
    invalidateQueriesMock.mockReset();
    debugMock.mockReset();
    getAccessTokenMock.mockReset().mockResolvedValue("access-token");
    getIdTokenMock.mockReset().mockResolvedValue("id-token");
    apiConfigMock.streamUrl = "https://stream.example.test";
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("does not connect when caseId is unset", async () => {
    renderHook(() => useCaseActivityStream(undefined), { wrapper });
    await Promise.resolve();
    expect(mockInstances).toHaveLength(0);
  });

  it("does not connect when the stream base URL isn't configured", async () => {
    apiConfigMock.streamUrl = undefined;
    renderHook(() => useCaseActivityStream("case-1"), { wrapper });
    await Promise.resolve();
    expect(mockInstances).toHaveLength(0);
  });

  it("connects with the case's stream URL and auth headers", async () => {
    renderHook(() => useCaseActivityStream("case-1"), { wrapper });

    await waitFor(() => expect(mockInstances).toHaveLength(1));
    const source = mockInstances[0];
    expect(source.url).toBe("https://stream.example.test/cases/case-1/activities/stream");
    expect(source.headers).toEqual({
      "x-jwt-assertion": "access-token",
      "x-user-id-token": "id-token",
    });
  });

  it("invalidates the comments and activities queries on a case_updated event", async () => {
    renderHook(() => useCaseActivityStream("case-1"), { wrapper });
    await waitFor(() => expect(mockInstances).toHaveLength(1));
    const source = mockInstances[0];

    act(() => {
      source.dispatchEvent(new MessageEvent("case_updated", { data: "{}" }));
    });

    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["csm-case-comments", "case-1"] }),
    );
    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["csm-case-activities", "case-1"] }),
    );
  });

  it("closes and reconnects with a fresh token after an error", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    renderHook(() => useCaseActivityStream("case-1"), { wrapper });
    await waitFor(() => expect(mockInstances).toHaveLength(1));
    const first = mockInstances[0];

    getAccessTokenMock.mockResolvedValue("fresh-access-token");
    act(() => {
      first.dispatchEvent(new Event("error"));
    });
    expect(first.closed).toBe(true);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3_000);
    });

    await waitFor(() => expect(mockInstances).toHaveLength(2));
    expect(mockInstances[1].headers?.["x-jwt-assertion"]).toBe("fresh-access-token");
  });

  it("closes the connection on unmount", async () => {
    const { unmount } = renderHook(() => useCaseActivityStream("case-1"), { wrapper });
    await waitFor(() => expect(mockInstances).toHaveLength(1));
    const source = mockInstances[0];

    unmount();

    expect(source.closed).toBe(true);
  });
});
