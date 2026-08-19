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

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { createRef } from "react";
import { useElementVisibleOnce } from "@hooks/useElementVisibleOnce";

/** jsdom doesn't implement `IntersectionObserver` at all (no precedent for
 * mocking it elsewhere in this codebase — checked). A minimal controllable
 * stub: each instance records its own callback/options and exposes a way
 * for a test to fire an intersection event manually, standing in for the
 * browser actually scrolling the element into view. */
class FakeIntersectionObserver {
  static instances: FakeIntersectionObserver[] = [];
  observedNodes: Element[] = [];
  disconnectCalls = 0;

  constructor(
    public callback: IntersectionObserverCallback,
    public options?: IntersectionObserverInit,
  ) {
    FakeIntersectionObserver.instances.push(this);
  }

  observe(node: Element): void {
    this.observedNodes.push(node);
  }

  unobserve(node: Element): void {
    this.observedNodes = this.observedNodes.filter((n) => n !== node);
  }

  disconnect(): void {
    this.disconnectCalls += 1;
    this.observedNodes = [];
  }

  /** Simulates the browser reporting `node` as intersecting (or not). */
  triggerIntersection(node: Element, isIntersecting: boolean): void {
    this.callback(
      [{ isIntersecting, target: node } as IntersectionObserverEntry],
      this as unknown as IntersectionObserver,
    );
  }
}

describe("useElementVisibleOnce", () => {
  const originalIntersectionObserver = globalThis.IntersectionObserver;

  beforeEach(() => {
    FakeIntersectionObserver.instances = [];
    (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver =
      FakeIntersectionObserver;
  });

  afterEach(() => {
    (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver =
      originalIntersectionObserver;
  });

  it("starts false and stays false until the observed element actually intersects", () => {
    const ref = createRef<HTMLDivElement>();
    // @ts-expect-error -- a bare object stands in for the DOM node; the
    // hook only ever passes it back to observe()/the callback, never reads
    // properties off it itself.
    ref.current = { tagName: "DIV" };

    const { result } = renderHook(() => useElementVisibleOnce(ref));

    expect(result.current).toBe(false);
    expect(FakeIntersectionObserver.instances).toHaveLength(1);
    expect(FakeIntersectionObserver.instances[0].observedNodes).toEqual([ref.current]);
  });

  it("passes the given rootMargin/threshold through to the observer (default 0px / 0.25)", () => {
    const ref = createRef<HTMLDivElement>();
    // @ts-expect-error -- see above.
    ref.current = { tagName: "DIV" };

    renderHook(() => useElementVisibleOnce(ref));

    // No pre-fetch-ahead-of-scroll margin, and at least a quarter of the
    // tile's own area must be on screen — see the constants' own doc
    // comments for why (an edge sliver, or a widget still below the fold
    // that a generous margin used to count as "visible", must not fire a
    // fetch).
    expect(FakeIntersectionObserver.instances[0].options).toEqual({
      rootMargin: "0px",
      threshold: 0.25,
    });
  });

  it("honors a caller-supplied rootMargin and threshold", () => {
    const ref = createRef<HTMLDivElement>();
    // @ts-expect-error -- see above.
    ref.current = { tagName: "DIV" };

    renderHook(() => useElementVisibleOnce(ref, "50px", 0.5));

    expect(FakeIntersectionObserver.instances[0].options).toEqual({
      rootMargin: "50px",
      threshold: 0.5,
    });
  });

  it("flips to true and disconnects once the element intersects, and never re-observes after", () => {
    const ref = createRef<HTMLDivElement>();
    // @ts-expect-error -- see above.
    ref.current = { tagName: "DIV" };

    const { result, rerender } = renderHook(() => useElementVisibleOnce(ref));
    const observer = FakeIntersectionObserver.instances[0];

    expect(result.current).toBe(false);

    act(() => {
      observer.triggerIntersection(ref.current as unknown as Element, true);
    });
    rerender();

    expect(result.current).toBe(true);
    expect(observer.disconnectCalls).toBe(1);

    // A later re-render must not spin up a second observer on an
    // already-latched element.
    rerender();
    expect(FakeIntersectionObserver.instances).toHaveLength(1);
  });

  it("ignores a non-intersecting entry and keeps waiting", () => {
    const ref = createRef<HTMLDivElement>();
    // @ts-expect-error -- see above.
    ref.current = { tagName: "DIV" };

    const { result, rerender } = renderHook(() => useElementVisibleOnce(ref));
    const observer = FakeIntersectionObserver.instances[0];

    act(() => {
      observer.triggerIntersection(ref.current as unknown as Element, false);
    });
    rerender();

    expect(result.current).toBe(false);
    expect(observer.disconnectCalls).toBe(0);
  });

  it("falls back to always-visible (true) when IntersectionObserver doesn't exist in this runtime", () => {
    (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver = undefined;
    const ref = createRef<HTMLDivElement>();
    // @ts-expect-error -- see above.
    ref.current = { tagName: "DIV" };

    const { result } = renderHook(() => useElementVisibleOnce(ref));

    expect(result.current).toBe(true);
    expect(FakeIntersectionObserver.instances).toHaveLength(0);
  });

  it("disconnects the observer on unmount even if the element never intersected", () => {
    const ref = createRef<HTMLDivElement>();
    // @ts-expect-error -- see above.
    ref.current = { tagName: "DIV" };

    const { unmount } = renderHook(() => useElementVisibleOnce(ref));
    const observer = FakeIntersectionObserver.instances[0];

    unmount();

    expect(observer.disconnectCalls).toBe(1);
  });
});
