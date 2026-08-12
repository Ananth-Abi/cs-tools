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
 * `1` — fully sequential, one widget fetch in flight at a time. This
 * supersedes an earlier `6` (chosen to match Chrome's own default
 * per-origin HTTP/1.1 connection limit, roughly halving the worst observed
 * prod burst while letting most dashboards render with barely perceptible
 * delay): the user explicitly asked for strictly one-by-one loading on the
 * abt-lead dashboard after checking `6` live, prioritizing backend-load
 * reduction over dashboard render speed. `1` costs the most render-latency
 * of any value here — a 20-widget dashboard now takes 20 sequential round
 * trips instead of ~4 batches of 6 — which is the explicit trade the user
 * chose, not an oversight. Tune here only; nothing else in the widget-fetch
 * path should hardcode a concurrency number.
 */
export const WIDGET_FETCH_CONCURRENCY_LIMIT = 1;

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

/**
 * Test-only escape hatch: clears every held/queued slot. `activeCount` and
 * `waiters` are module-level (deliberately — the cap is app-wide, not
 * per-dashboard), which means a test that mounts a widget whose fetch is
 * left permanently pending (`postMock.mockReturnValue(new Promise(() => {}))`,
 * a real pattern this codebase uses to assert a loading state) never
 * releases the slot it acquired — harmless at a cap of 6 (five slots still
 * free for the rest of that test file), but at a cap of 1 it permanently
 * starves every OTHER test in the same file, since there is nothing left
 * to acquire. Call this in a `beforeEach`/`afterEach` in any test file that
 * exercises a widget whose fetch may be left unresolved.
 */
export function __resetWidgetFetchConcurrencyForTests(): void {
  activeCount = 0;
  waiters.length = 0;
}
