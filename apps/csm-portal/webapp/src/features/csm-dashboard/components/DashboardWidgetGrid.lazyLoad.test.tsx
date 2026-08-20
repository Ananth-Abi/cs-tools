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
 * Proves the viewport-gated (lazy) loading behaviour end to end, through
 * the real fan-out component (`DashboardWidgetGrid`, one `.map()` over
 * every widget) rather than only unit-testing `useElementVisibleOnce` in
 * isolation: mounts a dashboard's worth of widgets under a mocked
 * `IntersectionObserver`, asserts only the ones reported as already
 * intersecting fetch immediately, then simulates the browser reporting an
 * off-screen one as newly intersecting (as it would once scrolled near the
 * viewport) and asserts THAT one fetches only at that point, not before.
 */

import { act, render, waitFor } from "@testing-library/react";
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

/** Same rationale as `useElementVisibleOnce.test.ts` — jsdom has no
 * `IntersectionObserver`, and this codebase has no existing mock for it, so
 * this is the first. Each observed node can be told "yes"/"no" individually
 * (real browser behaviour: only the entries that actually crossed the
 * threshold are reported), which is what lets this test simulate "some
 * widgets start in view, one starts out of view, then scrolls in". */
class FakeIntersectionObserver {
  static instances: FakeIntersectionObserver[] = [];
  callback: IntersectionObserverCallback;
  observedNodes = new Set<Element>();

  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback;
    FakeIntersectionObserver.instances.push(this);
  }

  observe(node: Element): void {
    this.observedNodes.add(node);
  }

  unobserve(node: Element): void {
    this.observedNodes.delete(node);
  }

  disconnect(): void {
    this.observedNodes.clear();
  }

  /** Fires only for `node` — mirrors the real API, which only reports
   * entries whose intersection state actually changed. */
  report(node: Element, isIntersecting: boolean): void {
    if (!this.observedNodes.has(node)) return;
    this.callback(
      [{ isIntersecting, target: node } as IntersectionObserverEntry],
      this as unknown as IntersectionObserver,
    );
  }
}

function widget(id: string, resourceType: BeDashboardWidget["resourceType"] = "case"): BeDashboardWidget {
  return {
    widgetId: id,
    displayName: id,
    resourceType,
    shape: "count",
    gridWidth: 3,
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

describe("DashboardWidgetGrid lazy widget loading", () => {
  const originalIntersectionObserver = globalThis.IntersectionObserver;

  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ total: 1, cases: [], limit: 1, offset: 0, hasMore: false });
    FakeIntersectionObserver.instances = [];
    (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver =
      FakeIntersectionObserver;
  });

  afterEach(() => {
    (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver =
      originalIntersectionObserver;
  });

  it("only fetches widgets already intersecting on mount; an off-screen widget fetches only once it later intersects", async () => {
    const widgets = [widget("above_fold_1"), widget("above_fold_2"), widget("below_fold")];
    const { container } = renderGrid(widgets);

    // Every tile registers its own observer instance (one ref each).
    await waitFor(() => expect(FakeIntersectionObserver.instances).toHaveLength(3));

    const cards = Array.from(container.querySelectorAll(".MuiCard-root"));
    expect(cards).toHaveLength(3);

    // Report the first two as already in view (as they'd be on a real
    // mount where they're above the fold) and leave the third alone —
    // exactly the "some widgets visible immediately, one is not" case.
    act(() => {
      FakeIntersectionObserver.instances[0].report(cards[0], true);
      FakeIntersectionObserver.instances[1].report(cards[1], true);
    });

    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(2));

    const fetchedIds = postMock.mock.calls.map(([, body]) => body);
    expect(fetchedIds).toHaveLength(2);
    // Give the (deliberately un-triggered) third tile a chance to have
    // fired if the gating didn't hold — it must still not have.
    expect(postMock).toHaveBeenCalledTimes(2);

    // Now simulate the user scrolling the third widget into view.
    act(() => {
      FakeIntersectionObserver.instances[2].report(cards[2], true);
    });

    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(3));
  });

  it("keeps a widget's data loaded once fetched, even if it were to report leaving the viewport again", async () => {
    const widgets = [widget("single")];
    const { container } = renderGrid(widgets);

    await waitFor(() => expect(FakeIntersectionObserver.instances).toHaveLength(1));
    const card = container.querySelector(".MuiCard-root") as Element;

    act(() => {
      FakeIntersectionObserver.instances[0].report(card, true);
    });
    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(1));

    // A real browser wouldn't even report this (the observer disconnects
    // itself on first intersection — see useElementVisibleOnce), but even
    // if it did, this hook's "visible once" latch must not un-fetch.
    act(() => {
      FakeIntersectionObserver.instances[0].report(card, false);
    });

    expect(postMock).toHaveBeenCalledTimes(1);
  });
});
