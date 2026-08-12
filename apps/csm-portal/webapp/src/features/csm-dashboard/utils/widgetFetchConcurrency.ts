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

/**
 * How many widget data-fetch requests (`useWidgetData` / `useWidgetPieData`)
 * may be in flight at once, across the whole app, not just one dashboard.
 *
 * A dashboard with N widgets previously fired N `/…/search` calls to
 * `customer-entity-service` essentially simultaneously on mount (one per
 * widget tile, each an independent `react-query` `queryFn`, no
 * coordination between them) — `abt-engineer` alone has ~20 widgets, and a
 * `shape: "pie"` widget fires one call per slice on top of that. Production
 * logs showed bursts of 5-13 simultaneous `Request timeout: POST
 * /cases/search` within ~170ms of each other on the entity-service, which
 * the BFF surfaced as HTTP 503 ("upstream connect error or disconnect/reset
 * before headers, reset reason: connection termination") at ~15s elapsed.
 *
 * 6 is chosen to match Chrome's own default per-origin HTTP/1.1 connection
 * limit — the browser would not have parallelized past that anyway on
 * HTTP/1.1 — while still letting most real dashboards (which have fewer
 * than 6 widgets, or complete their first 6 quickly) render with no
 * perceptible delay. It roughly halves the worst observed burst size (13)
 * without serializing small dashboards. Tune here only; nothing else in the
 * widget-fetch path should hardcode a concurrency number.
 */
export const WIDGET_FETCH_CONCURRENCY_LIMIT = 6;

let activeCount = 0;
const waiters: Array<() => void> = [];

function acquireWidgetFetchSlot(): Promise<void> {
  if (activeCount < WIDGET_FETCH_CONCURRENCY_LIMIT) {
    activeCount += 1;
    return Promise.resolve();
  }
  return new Promise<void>((resolve) => {
    waiters.push(() => {
      activeCount += 1;
      resolve();
    });
  });
}

function releaseWidgetFetchSlot(): void {
  activeCount = Math.max(0, activeCount - 1);
  const next = waiters.shift();
  if (next) next();
}

/**
 * Runs `fn` once a widget-fetch slot is free, releasing the slot as soon as
 * `fn` settles (success or failure) so the next queued fetch can start.
 * Callers past the cap simply await longer before `fn` starts — no error,
 * no change to `fn`'s own result or to `react-query`'s loading/error state,
 * which is exactly what a queued-but-not-yet-fired widget should show:
 * its normal loading skeleton, same as an in-flight one.
 *
 * A hand-rolled FIFO semaphore rather than a dependency (`p-limit` etc.) —
 * neither is already in `package.json`, and the mechanism this needs is a
 * dozen lines.
 */
export async function withWidgetFetchSlot<T>(fn: () => Promise<T>): Promise<T> {
  await acquireWidgetFetchSlot();
  try {
    return await fn();
  } finally {
    releaseWidgetFetchSlot();
  }
}
