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

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();
const getMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useQuickCaseSearch.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock, get: getMock }),
}));

import { useSearchTags } from "@features/csm-cases/api/useSearchTags";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useSearchTags", () => {
  beforeEach(() => {
    postMock.mockReset();
    getMock.mockReset();
    postMock.mockResolvedValue({ tags: [] });
  });

  // Serialising the captured payload is deliberate: comparing against the exact
  // JSON that goes on the wire catches a silent key rename, which an
  // objectContaining match on a decoded shape would happily accept.
  it("POSTs the query nested under filters.searchQuery with a top-level limit", async () => {
    const { result } = renderHook(() => useSearchTags("micro", true), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getMock).not.toHaveBeenCalled();
    expect(postMock).toHaveBeenCalledTimes(1);
    const [path, payload] = postMock.mock.calls[0];
    expect(path).toBe("/tags/search");
    expect(JSON.stringify(payload)).toBe(
      '{"filters":{"searchQuery":"micro"},"limit":20}',
    );
  });

  it("sends an empty searchQuery rather than omitting the filters object", async () => {
    const { result } = renderHook(() => useSearchTags("   ", true), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(JSON.stringify(postMock.mock.calls[0][1])).toBe(
      '{"filters":{"searchQuery":""},"limit":20}',
    );
  });

  it("returns the tags array from the unchanged {tags:[...]} response envelope", async () => {
    postMock.mockResolvedValue({
      tags: [{ id: "t1", label: "micro-gw", color: "#f97316" }],
    });

    const { result } = renderHook(() => useSearchTags("micro", true), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([
      { id: "t1", label: "micro-gw", color: "#f97316" },
    ]);
  });

  it("does not call the backend while disabled", () => {
    renderHook(() => useSearchTags("micro", false), { wrapper });
    expect(postMock).not.toHaveBeenCalled();
  });
});
