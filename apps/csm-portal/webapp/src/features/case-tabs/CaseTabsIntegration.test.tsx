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
import { describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { BrowserRouter, Route, Routes, useNavigate, useParams } from "react-router";
import "@testing-library/jest-dom/vitest";
import { useCaseRouteOverride } from "@context/case-tabs/CaseRouteOverrideContext";
import { CaseTabsProvider, useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import CaseDetailRouteSync from "@features/case-tabs/components/CaseDetailRouteSync";
import {
  CaseTabsContentHost,
  CaseTabStripBar,
} from "@features/case-tabs/components/CaseTabsWorkspace";
import { useReportCaseTabMeta } from "@features/case-tabs/hooks/useReportCaseTabMeta";
import { MAX_OPEN_CASE_TABS } from "@context/case-tabs/caseTabsTypes";

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
  useReportCaseTabMeta(caseId, label);
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
        <CaseTabsProvider>
          <CaseTabStripBar />
          <CaseTabsContentHost />
          <Routes>
            <Route path="/cases/:caseId" element={<CaseDetailRouteSync kind="case" />} />
          </Routes>
        </CaseTabsProvider>
      </ErrorBannerProvider>
    </BrowserRouter>
  );
}

describe("case tabs — real BrowserRouter integration", () => {
  it("opening a case via the tab mechanism renders it, without the nested-Router crash", async () => {
    expect(() => render(<App initialPath="/cases/CS0001" />)).not.toThrow();
    await waitFor(() =>
      expect(screen.getByTestId("stub-page-case-id")).toHaveTextContent("CS0001"),
    );
    // A tab for it is now open and shown in the strip.
    expect(screen.getByRole("tablist", { name: "Open cases" })).toBeInTheDocument();
  });

  // Regression test for bug: a tab's chip label stayed on the raw caseId
  // until the user switched to a different tab and back — i.e. it didn't
  // update in place while the tab stayed the active/visible one.
  it("bug 1 — a tab's label updates once its data resolves, without switching tabs away and back", async () => {
    render(<App initialPath="/cases/CS0001" />);
    await waitFor(() => screen.getByTestId("stub-page-case-id"));

    // Label not resolved yet: the chip falls back to the raw caseId.
    expect(screen.getByRole("tab")).toHaveTextContent("CS0001");

    // Simulate the page's data query resolving (matches
    // `CsmCaseDetailPage`'s own data -> label flow via `useReportCaseTabMeta`).
    fireEvent.click(screen.getByText("resolve-label"));

    await waitFor(() =>
      expect(screen.getByRole("tab")).toHaveTextContent("Label for CS0001"),
    );
    // Still exactly one tab, still the active one — this wasn't achieved by
    // closing/reopening or switching away.
    expect(screen.getAllByRole("tab")).toHaveLength(1);
  });

  // Regression test for bug: at the open-tab cap, clicking a 6th distinct
  // case silently kept showing a DIFFERENT, already-open tab's content (and
  // even redirected the URL to it) instead of the case that was actually
  // clicked.
  it("bug 2 — a 6th distinct case at the open-tab cap renders itself standalone, not a stale open tab", async () => {
    function AppWithFiveOpenTabs() {
      window.history.pushState({}, "", "/cases/CS0000");
      return (
        <BrowserRouter>
          <ErrorBannerProvider>
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
          </ErrorBannerProvider>
        </BrowserRouter>
      );
    }
    render(<AppWithFiveOpenTabs />);
    for (let i = 0; i < MAX_OPEN_CASE_TABS; i++) {
      fireEvent.click(screen.getByText(`open-CS000${i}`));
    }
    await waitFor(() => expect(screen.getAllByRole("tab")).toHaveLength(MAX_OPEN_CASE_TABS));

    // Navigate (as a case-list row click would) to a 6th, never-opened case.
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
    // Still exactly the original 5 tabs — the 6th case never became one.
    expect(screen.getAllByRole("tab")).toHaveLength(MAX_OPEN_CASE_TABS);
    // The user is told why.
    expect(screen.getByText(/already open/i)).toBeInTheDocument();
  });
});
