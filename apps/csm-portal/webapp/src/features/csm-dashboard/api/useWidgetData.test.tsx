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

import { render, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useWidgetData } from "@features/csm-dashboard/api/useWidgetData";
import { WIDGET_FETCH_CONCURRENCY_LIMIT } from "@features/csm-dashboard/utils/widgetFetchConcurrency";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

/** One dashboard tile: mounts its own independent useWidgetData query, same
 * as DashboardWidgetTile does per widget — the real fan-out this task caps. */
function Widget({ id }: { id: string }) {
  useWidgetData(id, "case", { states: ["open"] }, "count");
  return null;
}

function Dashboard({ widgetIds }: { widgetIds: string[] }) {
  return (
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      {widgetIds.map((id) => (
        <Widget key={id} id={id} />
      ))}
    </QueryClientProvider>
  );
}

describe("useWidgetData", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("issues one search for the widget's own filters, shape count uses limit 1", async () => {
    postMock.mockResolvedValue({ total: 7, items: [] });

    renderHook(() => useWidgetData("w1", "case", { states: ["open"] }, "count"), { wrapper });

    await waitFor(() => expect(postMock).toHaveBeenCalled());
    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: { states: ["open"] },
      pagination: { offset: 0, limit: 1 },
    });
  });

  it("caps concurrent in-flight /cases/search calls at WIDGET_FETCH_CONCURRENCY_LIMIT when a dashboard's worth of widgets all mount at once", async () => {
    // A dashboard with more widgets than abt-engineer's ~20 — deliberately
    // not a clean multiple of the cap.
    const widgetCount = WIDGET_FETCH_CONCURRENCY_LIMIT * 3 + 4;

    let inFlight = 0;
    let peakInFlight = 0;
    const releasers: Array<() => void> = [];

    postMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          inFlight += 1;
          peakInFlight = Math.max(peakInFlight, inFlight);
          releasers.push(() => {
            inFlight -= 1;
            resolve({ total: 1, items: [] });
          });
        }),
    );

    const widgetIds = Array.from({ length: widgetCount }, (_, i) => `w${i}`);
    render(<Dashboard widgetIds={widgetIds} />);

    // Let every widget's query mount and request a slot.
    await waitFor(() => expect(postMock.mock.calls.length).toBeGreaterThan(0));

    // Every widget beyond the cap must still be queued, not fired — the
    // core assertion this task exists to prove.
    expect(peakInFlight).toBeLessThanOrEqual(WIDGET_FETCH_CONCURRENCY_LIMIT);
    expect(postMock.mock.calls.length).toBeLessThanOrEqual(WIDGET_FETCH_CONCURRENCY_LIMIT);

    // Drain the queue in batches, exactly as a real backend finishing
    // requests over time would — the queued widgets fire in turn and the
    // cap continues to hold, until every widget has fetched.
    while (postMock.mock.calls.length < widgetCount || releasers.length > 0) {
      const batch = releasers.splice(0, releasers.length);
      batch.forEach((release) => release());
      await waitFor(() => expect(inFlight).toBe(0));
      expect(peakInFlight).toBeLessThanOrEqual(WIDGET_FETCH_CONCURRENCY_LIMIT);
      if (postMock.mock.calls.length >= widgetCount) break;
      await waitFor(() => expect(postMock.mock.calls.length).toBeGreaterThan(0));
    }

    await waitFor(() => expect(postMock.mock.calls.length).toBe(widgetCount));
    expect(peakInFlight).toBeLessThanOrEqual(WIDGET_FETCH_CONCURRENCY_LIMIT);
  });
});
