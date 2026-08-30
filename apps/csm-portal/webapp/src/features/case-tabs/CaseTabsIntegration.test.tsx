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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { BrowserRouter, Route, Routes, useNavigate, useParams } from "react-router";
import "@testing-library/jest-dom/vitest";
import { useCaseRouteOverride } from "@context/case-tabs/CaseRouteOverrideContext";
import { CaseTabsProvider, useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import {
  CaseTabsBehaviorProvider,
  useCaseTabsBehavior,
} from "@context/case-tabs/CaseTabsBehaviorContext";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import CaseDetailRouteSync from "@features/case-tabs/components/CaseDetailRouteSync";
import {
  CaseTabsContentHost,
  CaseTabStripBar,
} from "@features/case-tabs/components/CaseTabsWorkspace";
import { useReportCaseTabMeta } from "@features/case-tabs/hooks/useReportCaseTabMeta";
import { MAX_OPEN_CASE_TABS } from "@context/case-tabs/caseTabsTypes";

const ENABLED_STORAGE_KEY = "csm.caseTabs.enabled";
const CAP_MODE_STORAGE_KEY = "csm.caseTabs.capMode";

/**
 * Stand-in for `CsmCaseDetailPage`, wired in via the SAME lazy-loaded module
 * `App.tsx`'s real routes and `CaseTabsWorkspace`'s real keep-alive host
 * both import — so tests here exercise the actual production wiring end to
 * end (route match -> `CaseDetailRouteSync` -> `CaseTabsContext` ->
 * `CaseTabsContentHost`'s `CaseTabIsolatedRouter`), not a hand-rolled
 * substitute for it. Reports its own label via `useReportCaseTabMeta` the
 * exact same way the real page does now (see that hook's doc comment for
 * why), which is what makes the bug-1 regression test below meaningful.
 */
function StubCaseDetailPage() {
  const override = useCaseRouteOverride();
  const { caseId: routedCaseId } = useParams();
  const caseId = override?.caseId ?? routedCaseId;
  // A fake "still loading" -> "loaded" transition, matching a real
  // `useGetCsmCaseDetail` query — the label should appear once this
  // resolves, without needing the user to switch tabs away and back.
  const [label, setLabel] = useState<string | undefined>(undefined);
  useReportCaseTabMeta(caseId, { label, internalId: undefined, subject: undefined });
  return (
    <div>
      <div data-testid="stub-page-case-id">{caseId}</div>
      <button onClick={() => setLabel(`Label for ${caseId}`)}>resolve-label</button>
    </div>
  );
}

vi.mock("@features/case-tabs/tabPageRegistry", () => ({
  pageComponentForKind: () => StubCaseDetailPage,
}));

function Opener({ caseId }: { caseId: string }) {
  const { openTab } = useCaseTabsController();
  return <button onClick={() => openTab(caseId, "case", `/cases/${caseId}`)}>open-{caseId}</button>;
}

/** A real `<Link>`-equivalent click, same as a case-list row's own
 * navigation — exercises the real router, not a manual `history.pushState`
 * (which `BrowserRouter` doesn't observe the same way). */
function NavigateButton({ to, label }: { to: string; label: string }) {
  const navigate = useNavigate();
  return <button onClick={() => navigate(to)}>{label}</button>;
}

/**
 * End-to-end smoke test for the real `App.tsx`/`AppLayout` wiring shape:
 * one real `<BrowserRouter>` (as the app always has exactly one), the same
 * case-detail route pattern App.tsx registers, `CaseTabsContentHost`
 * (App.tsx and AppLayout render this in the app shell), and
 * `CaseDetailRouteSync` as the route's own element (App.tsx's actual
 * `element={<CaseDetailRouteSync kind="case" />}`).
 *
 * This is also the regression test for the "You cannot render a <Router>
 * inside another <Router>" crash a previous version of this feature shipped
 * with: that bug was only reachable through this exact real-router nesting,
 * which a standalone component render (this feature's other test files)
 * cannot reproduce.
 */
function App({ initialPath }: { initialPath: string }) {
  window.history.pushState({}, "", initialPath);
  return (
    <BrowserRouter>
      <ErrorBannerProvider>
        <CaseTabsBehaviorProvider>
          <CaseTabsProvider>
            <CaseTabStripBar />
            <CaseTabsContentHost />
            <Routes>
              <Route path="/cases/:caseId" element={<CaseDetailRouteSync kind="case" />} />
            </Routes>
          </CaseTabsProvider>
        </CaseTabsBehaviorProvider>
      </ErrorBannerProvider>
    </BrowserRouter>
  );
}

describe("case tabs — real BrowserRouter integration", () => {
  beforeEach(() => {
    // These tests exercise the tab mechanism itself; opt into a mode where
    // it's actually on (default is "off" — see the dedicated describe block
    // below for that).
    localStorage.setItem(ENABLED_STORAGE_KEY, "1");
    localStorage.setItem(CAP_MODE_STORAGE_KEY, "block");
  });

  it("opening a case via the tab mechanism renders it, without the nested-Router crash", async () => {
    expect(() => render(<App initialPath="/cases/CS0001" />)).not.toThrow();
    await waitFor(() =>
      expect(screen.getByTestId("stub-page-case-id")).toHaveTextContent("CS0001"),
    );
    // A tab for it is now open and shown in the strip, alongside the
    // permanent pinned "current location" tab (see `useCurrentLocationTab`).
    expect(screen.getByRole("tablist", { name: "Open cases" })).toBeInTheDocument();
    expect(screen.getByText("CS0001")).toBeInTheDocument();
  });

  // Regression test for bug: a tab's chip label stayed on the raw caseId
  // until the user switched to a different tab and back — i.e. it didn't
  // update in place while the tab stayed the active/visible one.
  it("bug 1 — a tab's label updates once its data resolves, without switching tabs away and back", async () => {
    render(<App initialPath="/cases/CS0001" />);
    await waitFor(() => screen.getByTestId("stub-page-case-id"));

    // Label not resolved yet: the chip falls back to "Loading…", not the
    // raw caseId/UUID.
    expect(screen.getByText("Loading…")).toBeInTheDocument();

    // Simulate the page's data query resolving (matches
    // `CsmCaseDetailPage`'s own data -> label flow via `useReportCaseTabMeta`).
    fireEvent.click(screen.getByText("resolve-label"));

    await waitFor(() => expect(screen.getByText("Label for CS0001")).toBeInTheDocument());
    // Still exactly one CASE tab (plus the permanent pinned one) — this
    // wasn't achieved by closing/reopening or switching away.
    expect(screen.getAllByRole("tab")).toHaveLength(2);
  });

  // Regression test for bug: at the open-tab cap, clicking one more distinct
  // case silently kept showing a DIFFERENT, already-open tab's content (and
  // even redirected the URL to it) instead of the case that was actually
  // clicked.
  it("bug 2 — a new distinct case past the cap renders itself standalone, not a stale open tab", async () => {
    function AppWithFiveOpenTabs() {
      window.history.pushState({}, "", "/cases/CS0000");
      return (
        <BrowserRouter>
          <ErrorBannerProvider>
            <CaseTabsBehaviorProvider>
              <CaseTabsProvider>
                {Array.from({ length: MAX_OPEN_CASE_TABS }, (_, i) => (
                  <Opener key={i} caseId={`CS000${i}`} />
                ))}
                <NavigateButton to="/cases/CS-OVERFLOW" label="click-overflow-case" />
                <CaseTabStripBar />
                <CaseTabsContentHost />
                <Routes>
                  <Route path="/cases/:caseId" element={<CaseDetailRouteSync kind="case" />} />
                </Routes>
              </CaseTabsProvider>
            </CaseTabsBehaviorProvider>
          </ErrorBannerProvider>
        </BrowserRouter>
      );
    }
    render(<AppWithFiveOpenTabs />);
    for (let i = 0; i < MAX_OPEN_CASE_TABS; i++) {
      fireEvent.click(screen.getByText(`open-CS000${i}`));
    }
    // The 5 case tabs, plus the permanent pinned "current location" tab.
    await waitFor(() =>
      expect(screen.getAllByRole("tab")).toHaveLength(MAX_OPEN_CASE_TABS + 1),
    );

    // Navigate (as a case-list row click would) to one more, never-opened case.
    fireEvent.click(screen.getByText("click-overflow-case"));

    // The URL must stay on the case that was actually clicked — not get
    // silently redirected back to whichever tab was previously active.
    await waitFor(() => expect(window.location.pathname).toBe("/cases/CS-OVERFLOW"));
    // The overflow case's own (un-tabbed) content must be what's showing —
    // not a stale already-open tab's. (Every open tab's page stays mounted
    // in the background — see `CaseTabIsolatedRouter` — so there are
    // multiple `stub-page-case-id` nodes at this point; `getByText` narrows
    // to the one whose visible text is this specific, never-opened case.)
    await waitFor(() => expect(screen.getByText("CS-OVERFLOW")).toBeInTheDocument());
    // And it's rendered outside any tab panel — the un-tabbed fallback path,
    // not a (still hidden) `CaseTabIsolatedRouter` instance.
    expect(screen.getByText("CS-OVERFLOW").closest('[data-testid^="case-tab-panel-"]')).toBeNull();
    // Still exactly the original MAX_OPEN_CASE_TABS case tabs (+ the pinned one) — the extra
    // case never became one.
    expect(screen.getAllByRole("tab")).toHaveLength(MAX_OPEN_CASE_TABS + 1);
    // The user is told why.
    expect(screen.getByText(/already open/i)).toBeInTheDocument();
  });
});

/**
 * Regression test for the single most consequential behavioral change in
 * this feature: the case-tabs mechanism defaults to OFF. A fresh
 * browser/session (empty localStorage) must see exactly the pre-feature
 * behavior — no tab strip at all, and clicking into a case renders it
 * in place via the real route, full page at a time, with no open-tab
 * bookkeeping happening at all.
 */
describe("case tabs — default (mode 'off') behavior", () => {
  beforeEach(() => {
    localStorage.removeItem(ENABLED_STORAGE_KEY);
    localStorage.removeItem(CAP_MODE_STORAGE_KEY);
    // Isolation from the mode-"block" tests above, which persist open tabs
    // to sessionStorage — mode "off" must ignore any such leftovers (see
    // `CaseTabsProvider`'s own doc comment on this), but clearing it here
    // too keeps this describe block's own fixture state independent.
    sessionStorage.clear();
  });

  it("renders no tab strip and no pinned tab on a fresh session", async () => {
    render(<App initialPath="/cases/CS0001" />);
    await waitFor(() => expect(screen.getByTestId("stub-page-case-id")).toBeInTheDocument());
    expect(screen.queryByRole("tablist", { name: "Open cases" })).not.toBeInTheDocument();
    expect(screen.queryAllByRole("tab")).toHaveLength(0);
  });

  it("renders every case directly (plain full-page navigation), never opening a tab", async () => {
    function AppAtTwoCasesInTurn({ path }: { path: string }) {
      window.history.pushState({}, "", path);
      return (
        <BrowserRouter>
          <ErrorBannerProvider>
            <CaseTabsBehaviorProvider>
              <CaseTabsProvider>
                <CaseTabStripBar />
                <CaseTabsContentHost />
                <Routes>
                  <Route path="/cases/:caseId" element={<CaseDetailRouteSync kind="case" />} />
                </Routes>
              </CaseTabsProvider>
            </CaseTabsBehaviorProvider>
          </ErrorBannerProvider>
        </BrowserRouter>
      );
    }
    const first = render(<AppAtTwoCasesInTurn path="/cases/CS0001" />);
    await waitFor(() =>
      expect(first.getByTestId("stub-page-case-id")).toHaveTextContent("CS0001"),
    );
    expect(first.queryByRole("tablist")).not.toBeInTheDocument();
    first.unmount();

    // A second, distinct case renders the same way — no accumulated tab
    // state carried between them (there IS none, in this mode).
    const second = render(<AppAtTwoCasesInTurn path="/cases/CS0002" />);
    await waitFor(() =>
      expect(second.getByTestId("stub-page-case-id")).toHaveTextContent("CS0002"),
    );
    expect(second.queryByRole("tablist")).not.toBeInTheDocument();
  });

  it("never shows the 'tabs are already open' toast in this mode", async () => {
    render(<App initialPath="/cases/CS0001" />);
    await waitFor(() => expect(screen.getByTestId("stub-page-case-id")).toBeInTheDocument());
    expect(screen.queryByText(/already open/i)).not.toBeInTheDocument();
  });

  // Regression test for bug: enabling the mechanism (mode "off" -> a mode
  // where it's on) while already viewing a case, in the SAME commit, used to
  // show a false-positive "tabs are already open" toast with zero tabs
  // actually open. Root cause: `CaseDetailRouteSync` (a descendant of
  // `CaseTabsProvider`) read the fresh `enabled` value directly and called
  // `openTab` from its own effect in that same commit, but React flushes
  // descendant effects before ancestor effects — so `CaseTabsProvider`'s own
  // effect hadn't yet synced its `enabledRef` from `false` to `true`, and
  // `openTab` saw the stale ref and refused, indistinguishable (to the
  // caller) from a genuine capacity refusal. Fixed by having `openTab` read
  // `enabled`/`capMode` directly as `useCallback` dependencies (always
  // fresh for the render that created it) instead of via an effect-synced
  // ref — see `CaseTabsContext`'s own comment on `openTab` for why a
  // render-time ref mutation wasn't a valid alternative here.
  it("enabling tabs while already on a case route opens a tab immediately, with no false capacity toast", async () => {
    function EnableToggle() {
      const { setEnabled } = useCaseTabsBehavior();
      return <button onClick={() => setEnabled(true)}>enable-tabs</button>;
    }
    function AppStartingDisabled({ initialPath }: { initialPath: string }) {
      window.history.pushState({}, "", initialPath);
      return (
        <BrowserRouter>
          <ErrorBannerProvider>
            <CaseTabsBehaviorProvider>
              <CaseTabsProvider>
                <EnableToggle />
                <CaseTabStripBar />
                <CaseTabsContentHost />
                <Routes>
                  <Route path="/cases/:caseId" element={<CaseDetailRouteSync kind="case" />} />
                </Routes>
              </CaseTabsProvider>
            </CaseTabsBehaviorProvider>
          </ErrorBannerProvider>
        </BrowserRouter>
      );
    }

    localStorage.removeItem(ENABLED_STORAGE_KEY);
    localStorage.setItem(CAP_MODE_STORAGE_KEY, "block");
    sessionStorage.clear();

    render(<AppStartingDisabled initialPath="/cases/CS0001" />);
    await waitFor(() => expect(screen.getByTestId("stub-page-case-id")).toBeInTheDocument());
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("enable-tabs"));

    await waitFor(() =>
      expect(screen.getByRole("tablist", { name: "Open cases" })).toBeInTheDocument(),
    );
    expect(screen.getByText("CS0001")).toBeInTheDocument();
    expect(screen.queryByText(/already open/i)).not.toBeInTheDocument();
  });
});
