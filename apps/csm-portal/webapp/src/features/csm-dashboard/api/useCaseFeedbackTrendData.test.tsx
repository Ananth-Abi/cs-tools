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

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useCaseFeedbackTrendData } from "@features/csm-dashboard/api/useCaseFeedbackTrendData";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useCaseFeedbackTrendData", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("issues a single date-bucketed aggregate request and maps buckets into non-navigable avg-rating slices", async () => {
    postMock.mockResolvedValue({
      buckets: [
        { bucketStart: "2026-07-01", avgRating: 4.2, count: 30 },
        { bucketStart: "2026-08-01", avgRating: 3.8, count: 15 },
      ],
      totalRecords: 45,
    });

    const { result } = renderHook(
      () =>
        useCaseFeedbackTrendData("feedback_trend", { accountIds: ["acc-1"] }, { bucket: "month" }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith(
      "/cases/feedback/aggregate",
      { filters: { accountIds: ["acc-1"] }, bucket: "month" },
      { signal: expect.any(AbortSignal) },
    );

    expect(result.current.slices).toEqual([
      { label: "Jul 2026", query: {}, navigable: false, value: 4.2 },
      { label: "Aug 2026", query: {}, navigable: false, value: 3.8 },
    ]);
    expect(result.current.total).toBe(45);
  });

  it("formats day/week buckets without a year", async () => {
    postMock.mockResolvedValue({
      buckets: [{ bucketStart: "2026-08-03", avgRating: 5, count: 2 }],
      totalRecords: 2,
    });

    const { result } = renderHook(
      () => useCaseFeedbackTrendData("feedback_trend", {}, { bucket: "day" }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.slices).toEqual([
      { label: "Aug 3", query: {}, navigable: false, value: 5 },
    ]);
  });

  it("fires no query and returns a zero result when groupBy is undefined", () => {
    const { result } = renderHook(
      () => useCaseFeedbackTrendData("feedback_trend", {}, undefined),
      { wrapper },
    );

    expect(postMock).not.toHaveBeenCalled();
    expect(result.current.slices).toEqual([]);
    expect(result.current.total).toBe(0);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.isError).toBe(false);
  });

  it("fires no query when groupBy carries field instead of bucket", () => {
    const { result } = renderHook(
      () => useCaseFeedbackTrendData("feedback_trend", {}, { field: "severity" }),
      { wrapper },
    );

    expect(postMock).not.toHaveBeenCalled();
    expect(result.current.slices).toEqual([]);
  });

  it("does not fire while enabled is false", () => {
    const { result } = renderHook(
      () => useCaseFeedbackTrendData("feedback_trend", {}, { bucket: "month" }, false),
      { wrapper },
    );

    expect(postMock).not.toHaveBeenCalled();
    expect(result.current.isLoading).toBe(true);
  });

  it("surfaces isError when the aggregate request fails", async () => {
    postMock.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(
      () => useCaseFeedbackTrendData("feedback_trend", {}, { bucket: "month" }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isError).toBe(true));
  });
});
