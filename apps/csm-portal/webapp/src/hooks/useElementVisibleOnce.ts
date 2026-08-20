// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { useEffect, useState, type RefObject } from "react";

/**
 * How far ahead of the viewport an element is treated as visible.
 *
 * Was `"200px"` — deliberately `"0px"` now. On the real abt-engineer
 * dashboard that 200px pre-fetch buffer, combined with `threshold: 0`
 * (the old implicit default — any nonzero overlap counts), meant a widget
 * row not yet meaningfully on screen still got counted as "visible" the
 * instant the dashboard mounted: with only 1-2 widgets actually on screen,
 * 4 fired their fetch. `"0px"` means only the real viewport counts — no
 * pre-fetch-ahead-of-scroll behavior anymore. That trades away the
 * "already loaded by the time you scroll to it" softness this hook
 * originally aimed for, but the user explicitly prioritized not
 * over-fetching over that convenience.
 */
const DEFAULT_ROOT_MARGIN = "0px";

/**
 * How much of the element's own area must overlap the (margin-adjusted)
 * root before it counts as visible. `0` (the implicit browser default)
 * means a single overlapping pixel at the viewport's edge — e.g. a grid
 * row that's 90% below the fold, with just its very top sliver crossing
 * into view — already counts as "visible". `0.25` requires at least a
 * quarter of the tile's own area on screen before its fetch fires, so an
 * edge sliver doesn't trigger it.
 */
const DEFAULT_THRESHOLD = 0.25;

/**
 * Tracks whether the given element has ever intersected the viewport (or
 * come within `rootMargin` of it, by at least `threshold` of its own
 * area), and keeps reporting `true` forever once it has — this is a
 * one-shot "has this been seen" latch, not a live on-screen/off-screen
 * toggle. Built for gating a widget tile's own data fetch (see
 * `DashboardWidgetTile`): once a tile has loaded its data, it must not
 * unmount/refetch just because the user scrolled it back out of view, so
 * the observer disconnects itself the first time it fires rather than
 * continuing to track subsequent intersections.
 *
 * Falls back to `true` (i.e. "always visible") when `IntersectionObserver`
 * doesn't exist in this runtime, rather than never firing — a widget in a
 * browser/environment without it should still load, just without the lazy
 * gating, which is strictly better than a widget stuck loading forever. The
 * same fallback also means test environments that don't stub
 * `IntersectionObserver` (jsdom doesn't implement it) see every widget as
 * immediately visible, matching this app's pre-lazy-load test expectations
 * without needing every existing test to add a mock.
 */
export function useElementVisibleOnce<T extends Element>(
  ref: RefObject<T | null>,
  rootMargin: string = DEFAULT_ROOT_MARGIN,
  threshold: number = DEFAULT_THRESHOLD,
): boolean {
  const [hasBeenVisible, setHasBeenVisible] = useState(
    () => typeof IntersectionObserver === "undefined",
  );

  useEffect(() => {
    if (hasBeenVisible) {
      // Either already latched true, or IntersectionObserver doesn't exist
      // in this runtime (see the initializer above) — nothing to observe.
      return;
    }
    const node = ref.current;
    if (!node) {
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setHasBeenVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin, threshold },
    );
    observer.observe(node);

    return () => {
      observer.disconnect();
    };
    // `hasBeenVisible`/`rootMargin`/`threshold` deliberately excluded:
    // re-running on `hasBeenVisible`'s own change would just re-observe an
    // already-latched node for no reason, and `rootMargin`/`threshold` are
    // effectively constants per call site. `ref` itself is a stable
    // RefObject — this effect keys on `ref.current` (the actual DOM node)
    // instead, so it re-observes if the tile's ref gets attached to a new
    // node.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ref.current]);

  return hasBeenVisible;
}
