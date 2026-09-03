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
 * Regression test for the live abt-engineer report: with the original
 * `rootMargin: "200px"` / implicit `threshold: 0`, only 1-2 widgets were
 * actually on screen when the dashboard opened, but 4 fired their fetch.
 * `useElementVisibleOnce.test.ts` covers the hook's own options in
 * isolation; this file proves the effect at the scale that actually
 * exposed the bug — a dashboard's worth of widgets (~20, abt-engineer's
 * own real count per this task's earlier investigation) in a real grid
 * layout — by simulating real element geometry (row position vs. a
 * simulated viewport height) instead of manually flipping
 * `isIntersecting` per tile as the other lazy-load test file does.
 *
 * Caveat, stated plainly rather than implied: this repo has no local copy
 * of abt-engineer's actual `DASHBOARDS_CONFIG` (it's backend-owned/env-var
 * config, not checked into this webapp, and pulling the live value would
 * mean hitting Choreo/prod, out of scope for a frontend test) and jsdom
 * has no real layout engine, so `getBoundingClientRect` on real rendered
 * nodes is useless here regardless. The geometry below (4 count-shape
 * tiles per row, 160px row pitch, 350px "above the fold" viewport height)
 * is a representative stand-in sized to plausibly match a real dashboard,
 * not a literal reproduction of the live numbers — what this test actually
 * proves is the mechanism: given a widget's real on-screen position, the
 * FIXED hook (0px margin, 0.25 threshold) only fires for widgets with
 * meaningful on-screen overlap, and demonstrably fewer of them than the
 * OLD settings would have. The live dev-server re-check is what confirms
 * the actual abt-engineer numbers.
 */

import { render, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router";
import type { BeDashboardWidget } from "@api/backend/types";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { id: "11111111-aaaa-bbbb-cccc-000000000001" },
    isLoading: false,
    isError: false,
  }),
}));

import DashboardWidgetGrid from "@features/csm-dashboard/components/DashboardWidgetGrid";

// --- Simulated layout -------------------------------------------------
// 4 widgets per row (gridWidth 3 of 12 — a common count-shape tile width
// on the real dashboards), pitch 160px per row (card height + grid gap),
// 20 total widgets (abt-engineer's own approximate count per this task's
// earlier prod-log investigation) — 5 rows. "Above the fold" viewport
// height 350px models a realistic content area once the app's own
// header/nav/dashboard-header chrome is subtracted from a normal laptop
// window.
const WIDGETS_PER_ROW = 4;
const ROW_HEIGHT_PX = 160;
const TOTAL_WIDGETS = 20;
const VIEWPORT_HEIGHT_PX = 350;

function rowTop(index: number): number {
  return Math.floor(index / WIDGETS_PER_ROW) * ROW_HEIGHT_PX;
}

/** Same overlap math the fake observer below uses per-node — exposed
 * separately so this file can assert the OLD-vs-NEW magnitude difference
 * as plain arithmetic, not just implicitly through the component mount. */
function isVisible(index: number, marginPx: number, threshold: number): boolean {
  const top = rowTop(index);
  const bottom = top + ROW_HEIGHT_PX;
  const rootTop = 0 - marginPx;
  const rootBottom = VIEWPORT_HEIGHT_PX + marginPx;
  const overlap = Math.max(0, Math.min(bottom, rootBottom) - Math.max(top, rootTop));
  const ratio = overlap / ROW_HEIGHT_PX;
  return overlap > 0 && ratio >= threshold;
}

function countVisible(marginPx: number, threshold: number): number {
  let count = 0;
  for (let i = 0; i < TOTAL_WIDGETS; i += 1) {
    if (isVisible(i, marginPx, threshold)) count += 1;
  }
  return count;
}

/** Computes real intersection geometry from each observed node's position
 * in `DashboardWidgetGrid`'s own render order (widgets mount in array
 * order, and their effects — which call `observe()` — run in that same
 * order; this is the assumption that lets index-based rects work without
 * jsdom's nonexistent real layout) rather than requiring a test to flip
 * `isIntersecting` by hand per tile, which is what makes this file able to
 * exercise all 20 widgets' worth of geometry at once. */
class GeometryIntersectionObserver {
  static instances: GeometryIntersectionObserver[] = [];
  static nextObserveIndex = 0;

  callback: IntersectionObserverCallback;
  marginPx: number;
  threshold: number;

  constructor(callback: IntersectionObserverCallback, options?: IntersectionObserverInit) {
    this.callback = callback;
    const match = /^(-?\d+)px$/.exec(String(options?.rootMargin ?? "0px").trim());
    this.marginPx = match ? Number(match[1]) : 0;
    this.threshold = typeof options?.threshold === "number" ? options.threshold : 0;
    GeometryIntersectionObserver.instances.push(this);
  }

  observe(node: Element): void {
    const index = GeometryIntersectionObserver.nextObserveIndex;
    GeometryIntersectionObserver.nextObserveIndex += 1;
    const visible = isVisible(index, this.marginPx, this.threshold);
    this.callback(
      [{ isIntersecting: visible, target: node } as IntersectionObserverEntry],
      this as unknown as IntersectionObserver,
    );
  }

  unobserve(): void {}
  disconnect(): void {}
}

function widget(index: number): BeDashboardWidget {
  return {
    widgetId: `widget_${index}`,
    displayName: `Widget ${index}`,
    resourceType: "case",
    shape: "count",
    gridWidth: 3, // 12 / 3 = 4 per row, matching WIDGETS_PER_ROW above.
    query: {},
  };
}

function renderGrid(widgets: BeDashboardWidget[]) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <DashboardWidgetGrid widgets={widgets} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("DashboardWidgetGrid lazy loading at realistic dashboard scale", () => {
  const originalIntersectionObserver = globalThis.IntersectionObserver;

  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ total: 1, cases: [], limit: 1, offset: 0, hasMore: false });
    GeometryIntersectionObserver.instances = [];
    GeometryIntersectionObserver.nextObserveIndex = 0;
    (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver =
      GeometryIntersectionObserver;
  });

  afterEach(() => {
    (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver =
      originalIntersectionObserver;
  });

  it("documents the old-vs-new magnitude: 200px/threshold-0 over-fires relative to what's genuinely on screen, 0px/0.25 does not", () => {
    // Genuinely on screen: rows whose full height fits within the 350px
    // viewport with no margin at all — the ground truth a human looking at
    // the screen would report.
    const genuinelyVisible = countVisible(0, 1); // threshold 1 == "fully on screen"
    expect(genuinelyVisible).toBe(8); // rows 0-1 (4 widgets each) fully fit; row 2 does not.

    const oldConfig = countVisible(200, 0);
    const newConfig = countVisible(0, 0.25);

    expect(oldConfig).toBe(16); // rows 0-2 fully "inside" the 200px-extended root, row 3 partially — this is the over-fetch bug.
    expect(newConfig).toBe(8); // matches genuinelyVisible exactly: rows 0-1 only.
    expect(newConfig).toBeLessThan(oldConfig);
  });

  it("the real DashboardWidgetGrid, with the fixed hook's actual default options, fetches only the widgets with meaningful on-screen overlap — not more", async () => {
    const widgets = Array.from({ length: TOTAL_WIDGETS }, (_, i) => widget(i));
    renderGrid(widgets);

    await waitFor(() =>
      expect(GeometryIntersectionObserver.instances).toHaveLength(TOTAL_WIDGETS),
    );

    // Every widget's own useElementVisibleOnce call goes through this fake
    // observer with the hook's REAL default options (rootMargin "0px",
    // threshold 0.25) — nothing in this test hardcodes those values itself,
    // so a regression back to "200px"/no-threshold would change this
    // assertion's outcome, not just the isolated hook-options test.
    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(8));

    // Give any wrongly-gated widget a tick to have fired anyway.
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(postMock).toHaveBeenCalledTimes(8);
  });
});
