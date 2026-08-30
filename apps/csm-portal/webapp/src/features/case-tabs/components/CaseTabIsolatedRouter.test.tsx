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

import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { useRef, useState, type JSX } from "react";
import { BrowserRouter, useNavigate, useParams } from "react-router";
import "@testing-library/jest-dom/vitest";
import CaseTabIsolatedRouter from "@features/case-tabs/components/CaseTabIsolatedRouter";
import { CaseTabsProvider, useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import { CaseTabsBehaviorProvider } from "@context/case-tabs/CaseTabsBehaviorContext";
import { useCaseRouteOverride } from "@context/case-tabs/CaseRouteOverrideContext";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

/**
 * Stand-in for `CsmCaseDetailPage` — reads its `caseId`/`navigate` the exact
 * same way the real page does post-fix: real router hooks called
 * unconditionally, with `useCaseRouteOverride()`'s value preferred when
 * present (see `CsmCaseDetailPage`'s own top-of-component comment). Keeping
 * this independent of the (very large) real page is what lets these tests
 * verify the tab-isolation mechanism on its own — but reproducing the exact
 * hook-call shape is what makes the "inside a real BrowserRouter" tests
 * below actually exercise the bug class this file exists to catch: a naive
 * stub that doesn't call the real react-router hooks at all wouldn't have
 * caught the "second `<Router>` inside a `<Router>`" crash either.
 */
function StubCasePage(): JSX.Element {
  const override = useCaseRouteOverride();
  const { caseId: routedCaseId } = useParams();
  const routedNavigate = useNavigate();
  const caseId = override?.caseId ?? routedCaseId;
  const navigate = override?.navigate ?? routedNavigate;
  const [draft, setDraft] = useState("");
  return (
    <div>
      <div data-testid="stub-case-id">{caseId}</div>
      <input aria-label="draft" value={draft} onChange={(e) => setDraft(e.target.value)} />
      <button onClick={() => navigate("/cases/OTHER-CASE")}>go-to-other-case</button>
    </div>
  );
}

function Harness({ tabs, visibleId }: { tabs: CaseTabState[]; visibleId: string }): JSX.Element {
  return (
    <div>
      {tabs.map((tab) => (
        <CaseTabIsolatedRouter key={tab.id} tab={tab} isVisible={tab.id === visibleId}>
          <StubCasePage />
        </CaseTabIsolatedRouter>
      ))}
    </div>
  );
}

const TAB_A: CaseTabState = {
  id: "t-a",
  caseId: "CS1",
  kind: "case",
  path: "/cases/CS1",
  hasDraft: false,
};
const TAB_B: CaseTabState = {
  id: "t-b",
  caseId: "CS2",
  kind: "case",
  path: "/cases/CS2",
  hasDraft: false,
};

/**
 * Every render in this file is wrapped in a real `<BrowserRouter>` — the app
 * shell (`App.tsx`) always has exactly one, and mounting this mechanism
 * outside of one (as an earlier version of this test file did) is exactly
 * what let a real bug slip through: react-router's own invariant against
 * nesting a second `<Router>` inside another one only fires when there IS an
 * outer `<Router>` to collide with. A standalone render can't reproduce that
 * — see the regression test below, which exists specifically to catch this
 * bug class going forward.
 */
function renderInApp(ui: JSX.Element): ReturnType<typeof render> {
  return render(<BrowserRouter>{ui}</BrowserRouter>);
}

describe("CaseTabIsolatedRouter", () => {
  beforeEach(() => {
    // Only the last test below actually opens a tab via the real `openTab`
    // (the others hand-build `CaseTabState` objects directly) — but this is
    // harmless for those, and keeps this test independent of whatever the
    // default behavior mode is.
    localStorage.setItem("csm.caseTabs.enabled", "1");
  });

  it("does not throw react-router's nested-Router invariant when mounted inside the app's real BrowserRouter", () => {
    // This is the regression test for the bug this component previously
    // shipped with: an earlier implementation rendered a second, low-level
    // `<Router>` per tab, which crashes unconditionally the moment it's
    // mounted inside the app's real `<BrowserRouter>` (App.tsx) — a
    // standalone render (no outer Router at all) can't reproduce that crash,
    // which is exactly how it escaped the original test suite.
    expect(() =>
      renderInApp(
        <CaseTabsProvider>
          <Harness tabs={[TAB_A, TAB_B]} visibleId="t-a" />
        </CaseTabsProvider>,
      ),
    ).not.toThrow();
  });

  it("resolves the correct caseId per tab via the route override, not the real (single) router match", () => {
    renderInApp(
      <CaseTabsProvider>
        <Harness tabs={[TAB_A, TAB_B]} visibleId="t-a" />
      </CaseTabsProvider>,
    );
    const panels = screen.getAllByTestId("stub-case-id");
    expect(panels.map((p) => p.textContent)).toEqual(["CS1", "CS2"]);
  });

  it("hides the inactive tab's panel via CSS but keeps it mounted", () => {
    renderInApp(
      <CaseTabsProvider>
        <Harness tabs={[TAB_A, TAB_B]} visibleId="t-a" />
      </CaseTabsProvider>,
    );
    expect(screen.getByTestId("case-tab-panel-t-a")).not.toHaveAttribute("hidden");
    expect(screen.getByTestId("case-tab-panel-t-b")).toHaveAttribute("hidden");
    // Still in the DOM (mounted), just hidden — not removed.
    expect(screen.getByTestId("case-tab-panel-t-b")).toBeInTheDocument();
  });

  it("preserves an inactive tab's local state (draft text) across a re-render", () => {
    const { rerender } = renderInApp(
      <CaseTabsProvider>
        <Harness tabs={[TAB_A, TAB_B]} visibleId="t-a" />
      </CaseTabsProvider>,
    );
    const inputs = screen.getAllByLabelText("draft") as HTMLInputElement[];
    fireEvent.change(inputs[1], { target: { value: "unsent reply on CS2" } });
    expect(inputs[1].value).toBe("unsent reply on CS2");

    // Switch the visible tab (simulating the workspace re-rendering after
    // activating a different tab) — CaseTabIsolatedRouter for CS2 must NOT
    // remount (same React key `t-b`), so its draft text survives.
    rerender(
      <BrowserRouter>
        <CaseTabsProvider>
          <Harness tabs={[TAB_A, TAB_B]} visibleId="t-b" />
        </CaseTabsProvider>
      </BrowserRouter>,
    );
    const inputsAfter = screen.getAllByLabelText("draft") as HTMLInputElement[];
    expect(inputsAfter[1].value).toBe("unsent reply on CS2");
  });

  it("an in-page navigation to a different case opens/activates a new tab instead of retargeting this one", () => {
    function OpenerHarness(): JSX.Element {
      const { tabs, activeTabId, openTab } = useCaseTabsController();
      return (
        <div>
          <div data-testid="active-id">{activeTabId}</div>
          <div data-testid="tab-count">{tabs.length}</div>
          <button onClick={() => openTab("CS1", "case", "/cases/CS1")}>seed</button>
          {tabs.map((tab) => (
            <CaseTabIsolatedRouter key={tab.id} tab={tab} isVisible={tab.id === activeTabId}>
              <StubCasePage />
            </CaseTabIsolatedRouter>
          ))}
        </div>
      );
    }
    renderInApp(
      <CaseTabsBehaviorProvider>
        <CaseTabsProvider>
          <OpenerHarness />
        </CaseTabsProvider>
      </CaseTabsBehaviorProvider>,
    );
    fireEvent.click(screen.getByText("seed"));
    expect(screen.getByTestId("tab-count")).toHaveTextContent("1");
    fireEvent.click(screen.getByText("go-to-other-case"));
    expect(screen.getByTestId("tab-count")).toHaveTextContent("2");
    // The original tab (CS1) is untouched; the new one (OTHER-CASE) is now active.
    const ids = screen.getAllByTestId("stub-case-id").map((n) => n.textContent);
    expect(ids).toContain("CS1");
    expect(ids).toContain("OTHER-CASE");
  });

  // Regression test for bug: a tab's scroll position was lost on switching
  // away and back, even though its other state (drafts, dialogs) correctly
  // persisted — root cause was `AppLayout`'s single shared scroll container
  // (`mainContentRef`) being reset to the top on every route change,
  // including a tab-to-tab switch, since every open tab shared that SAME
  // element as its scroll container. Reproduced directly below rather than
  // importing the real (very large) `AppLayout`.
  it("switching to another tab and back doesn't touch this tab's own scroll position, unaffected by a shared sibling scroll container's reset", () => {
    function AppLayoutStyleHarness() {
      const [visibleId, setVisibleId] = useState<string>("t-a");
      // Stands in for `AppLayout`'s own `mainContentRef` + its
      // reset-on-route-change effect — a SIBLING element, not an ancestor
      // of the tab panels, same relationship as the real `AppLayout`
      // (`mainContentRef` wraps `CaseTabsContentHost`, but is not itself
      // inside any of `CaseTabIsolatedRouter`'s per-tab panels).
      const sharedScrollRef = useRef<HTMLDivElement>(null);
      return (
        <div>
          <div ref={sharedScrollRef} data-testid="shared-scroll-container" />
          <button onClick={() => setVisibleId("t-a")}>show-a</button>
          <button onClick={() => setVisibleId("t-b")}>show-b</button>
          <button
            onClick={() => {
              // The exact reset this bug's root cause performs — applied to
              // the SHARED sibling container, never to either tab's own
              // panel.
              if (sharedScrollRef.current) sharedScrollRef.current.scrollTop = 0;
            }}
          >
            simulate-route-change-scroll-reset
          </button>
          <Harness tabs={[TAB_A, TAB_B]} visibleId={visibleId} />
        </div>
      );
    }
    renderInApp(
      <CaseTabsProvider>
        <AppLayoutStyleHarness />
      </CaseTabsProvider>,
    );

    const panelA = screen.getByTestId("case-tab-panel-t-a");
    panelA.scrollTop = 240;
    // `handleScroll` (the explicit capture the follow-up test below covers
    // in more depth) is what actually records this — a native `scroll`
    // event, not just the property assignment above.
    fireEvent.scroll(panelA);
    expect(panelA.scrollTop).toBe(240);

    // Switch to tab B — a route change in the real app, so the shared
    // container's own reset fires too (simulated explicitly here).
    fireEvent.click(screen.getByText("show-b"));
    fireEvent.click(screen.getByText("simulate-route-change-scroll-reset"));

    // Switch back to tab A.
    fireEvent.click(screen.getByText("show-a"));

    // Tab A's own scroll position survived — it was never on the shared
    // container the reset actually touched.
    expect(screen.getByTestId("case-tab-panel-t-a").scrollTop).toBe(240);
  });

  // Regression test for a follow-up bug in the fix above: relying on the
  // browser to remember a hidden (`display: none`) element's own `scrollTop`
  // isn't spec-guaranteed — per the CSSOM View spec an element with no
  // layout box has no defined `scrollTop`, and browsers are inconsistent
  // about restoring it once it becomes visible again (some reset it to 0;
  // see the WPT test `scrollTop-display-change.html`). This is why the
  // position is captured on scroll (`handleScroll`, continuously, not just
  // "on hide" — by the time a `display: none` commit lands this panel's own
  // `scrollTop` may already be unreadable) and explicitly reapplied by a
  // layout effect the instant the panel becomes visible again, rather than
  // trusting passive DOM persistence. Proven here by deliberately zeroing
  // the panel's `scrollTop` WHILE IT'S HIDDEN (standing in for a browser
  // that does reset it) — passive persistence alone would show 0 after
  // switching back; the explicit restore must still produce 240.
  it("explicitly reapplies a tab's saved scroll position on becoming visible again, even if the browser reset it while hidden", () => {
    const { rerender } = renderInApp(
      <CaseTabsProvider>
        <Harness tabs={[TAB_A, TAB_B]} visibleId="t-a" />
      </CaseTabsProvider>,
    );

    const panelA = screen.getByTestId("case-tab-panel-t-a") as HTMLDivElement;
    panelA.scrollTop = 240;
    // `handleScroll` is what actually captures this into `savedScrollTopRef`
    // — a plain property assignment (as in the test above) doesn't fire a
    // native `scroll` event on its own in jsdom.
    fireEvent.scroll(panelA);

    // Hide tab A's panel the same way switching tabs does (`isVisible`
    // false -> the real component sets `hidden`/`display: none`) — this
    // test drives that via `rerender` since `Harness` takes `visibleId` as
    // a prop, same mechanism `CaseTabsContentHost` itself uses. Kept inside
    // the same `<BrowserRouter>` wrapper `renderInApp` used initially — the
    // stub page underneath still calls the real `useParams`/`useNavigate`.
    rerender(
      <BrowserRouter>
        <CaseTabsProvider>
          <Harness tabs={[TAB_A, TAB_B]} visibleId="t-b" />
        </CaseTabsProvider>
      </BrowserRouter>,
    );

    // Simulate a browser that does NOT preserve a hidden element's
    // `scrollTop` (the CSSOM-View-ambiguous case CodeRabbit flagged) —
    // without the explicit restore below, the panel would still read 0
    // after switching back.
    panelA.scrollTop = 0;

    rerender(
      <BrowserRouter>
        <CaseTabsProvider>
          <Harness tabs={[TAB_A, TAB_B]} visibleId="t-a" />
        </CaseTabsProvider>
      </BrowserRouter>,
    );

    expect(panelA.scrollTop).toBe(240);
  });

  it("each tab's own panel is its own scroll container (overflowY: auto)", () => {
    renderInApp(
      <CaseTabsProvider>
        <Harness tabs={[TAB_A, TAB_B]} visibleId="t-a" />
      </CaseTabsProvider>,
    );
    expect(screen.getByTestId("case-tab-panel-t-a")).toHaveStyle({ overflowY: "auto" });
    expect(screen.getByTestId("case-tab-panel-t-b")).toHaveStyle({ overflowY: "auto" });
  });
});
