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

import { describe, expect, it } from "vitest";
import {
  WIDGET_FETCH_CONCURRENCY_LIMIT,
  withWidgetFetchSlot,
} from "@features/csm-dashboard/utils/widgetFetchConcurrency";

describe("withWidgetFetchSlot", () => {
  it("never lets more than WIDGET_FETCH_CONCURRENCY_LIMIT calls run at once, even when far more are requested simultaneously", async () => {
    const totalRequests = WIDGET_FETCH_CONCURRENCY_LIMIT * 4 + 3; // deliberately not a clean multiple
    let concurrent = 0;
    let peakConcurrent = 0;
    const started: number[] = [];

    // Every "widget" resolves only once every one of them has requested a
    // slot — the same shape as N dashboard tiles all mounting and firing
    // their query at once. If the cap didn't hold, every one of these
    // would run its body immediately and peakConcurrent would equal
    // totalRequests.
    const releaseGate = () => new Promise<void>((resolve) => setTimeout(resolve, 5));

    const calls = Array.from({ length: totalRequests }, (_, i) =>
      withWidgetFetchSlot(async () => {
        started.push(i);
        concurrent += 1;
        peakConcurrent = Math.max(peakConcurrent, concurrent);
        await releaseGate();
        concurrent -= 1;
        return i;
      }),
    );

    const results = await Promise.all(calls);

    expect(results).toHaveLength(totalRequests);
    expect(peakConcurrent).toBeLessThanOrEqual(WIDGET_FETCH_CONCURRENCY_LIMIT);
    expect(peakConcurrent).toBeGreaterThan(0);
    // With a real gate every call has to wait through, the cap should
    // actually bind at least once — otherwise this test would pass
    // vacuously even with no cap at all if totalRequests were small.
    expect(peakConcurrent).toBe(WIDGET_FETCH_CONCURRENCY_LIMIT);
  });

  it("releases a slot as soon as fn rejects, so a failing widget doesn't starve the queue", async () => {
    const results: string[] = [];

    const failing = withWidgetFetchSlot(async () => {
      throw new Error("widget search failed");
    }).catch(() => {
      results.push("failed");
    });

    // Fill every remaining slot with fast successes.
    const fillers = Array.from({ length: WIDGET_FETCH_CONCURRENCY_LIMIT - 1 }, (_, i) =>
      withWidgetFetchSlot(async () => {
        results.push(`filler-${i}`);
      }),
    );

    // One more, queued past the cap — only starts once a slot frees up,
    // which requires the failed call above to have released its slot.
    const queued = withWidgetFetchSlot(async () => {
      results.push("queued");
    });

    await Promise.all([failing, ...fillers, queued]);

    expect(results).toContain("failed");
    expect(results).toContain("queued");
  });

  it("returns fn's own resolved value unchanged", async () => {
    const value = await withWidgetFetchSlot(async () => ({ total: 42 }));
    expect(value).toEqual({ total: 42 });
  });

  it("propagates fn's own rejection unchanged", async () => {
    await expect(
      withWidgetFetchSlot(async () => {
        throw new Error("boom");
      }),
    ).rejects.toThrow("boom");
  });
});
