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

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useSearchGroups.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useSearchServiceRequestsForSelect } from "@features/csm-operations/api/useSearchServiceRequestsForSelect";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useSearchServiceRequestsForSelect", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("searches unscoped (type filter only) when no projectId is given", async () => {
    postMock.mockResolvedValue({
      cases: [{ id: "sr-1", number: "CS-0001", subject: "Reset admin password" }],
    });

    const { result } = renderHook(
      () => useSearchServiceRequestsForSelect("reset", true),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      expect.objectContaining({
        filters: expect.objectContaining({
          searchQuery: "reset",
          filters: [{ field: "type", op: "in", values: ["service_request"] }],
        }),
      }),
    );
    expect(result.current.data).toEqual([
      { id: "sr-1", number: "CS-0001", subject: "Reset admin password" },
    ]);
  });

  it("scopes the first search to projectId when given, and does not fall back when it finds matches", async () => {
    postMock.mockResolvedValue({
      cases: [{ id: "sr-1", number: "CS-0001", subject: "In-project match" }],
    });

    const { result } = renderHook(
      () => useSearchServiceRequestsForSelect("", true, "prj-1"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      expect.objectContaining({
        filters: expect.objectContaining({
          filters: [
            { field: "type", op: "in", values: ["service_request"] },
            { field: "projectId", op: "in", values: ["prj-1"] },
          ],
        }),
      }),
    );
    expect(result.current.data).toEqual([
      { id: "sr-1", number: "CS-0001", subject: "In-project match" },
    ]);
  });

  it("falls back to the unscoped, system-wide search when the projectId-scoped search finds nothing", async () => {
    postMock
      .mockResolvedValueOnce({ cases: [] })
      .mockResolvedValueOnce({
        cases: [{ id: "sr-9", number: "CS-0009", subject: "Different project" }],
      });

    const { result } = renderHook(
      () => useSearchServiceRequestsForSelect("", true, "prj-1"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // Scoped attempt first, then the unscoped fallback — never excludes a
    // real match just because it sits outside the scoped project.
    expect(postMock).toHaveBeenCalledTimes(2);
    expect(postMock.mock.calls[0][1].filters.filters).toEqual([
      { field: "type", op: "in", values: ["service_request"] },
      { field: "projectId", op: "in", values: ["prj-1"] },
    ]);
    expect(postMock.mock.calls[1][1].filters.filters).toEqual([
      { field: "type", op: "in", values: ["service_request"] },
    ]);
    expect(result.current.data).toEqual([
      { id: "sr-9", number: "CS-0009", subject: "Different project" },
    ]);
  });
});
