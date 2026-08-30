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

import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { BrowserRouter, Route, Routes } from "react-router";
import "@testing-library/jest-dom/vitest";
import { useParams } from "react-router";
import { useCaseRouteOverride } from "@context/case-tabs/CaseRouteOverrideContext";
import { CaseTabsProvider } from "@context/case-tabs/CaseTabsContext";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import CaseDetailRouteSync from "@features/case-tabs/components/CaseDetailRouteSync";
import {
  CaseTabsContentHost,
  CaseTabStripBar,
} from "@features/case-tabs/components/CaseTabsWorkspace";

// `CaseTabStripBar` resolves each open tab's label via the same
// `useGetCsmCaseDetail` query the real page uses (see `CaseTabLabel`), which
// reads backend config at module load — not present under vitest (same
// reason `CsmCaseDetailPage.test.tsx` mocks it). The tab label itself isn't
// what this test cares about, so a minimal stub is enough.
vi.mock("@features/csm-cases/api/useGetCsmCaseDetail", () => ({
  useGetCsmCaseDetail: () => ({ data: undefined, isLoading: false }),
}));

// Swaps out the real (very large) `CsmCaseDetailPage` for a tiny stub, the
// same way every other test in this feature does — but here that stub is
// wired in via the SAME lazy-loaded module `App.tsx`'s real routes and
// `CaseTabsWorkspace`'s real keep-alive host both import, so this test
// exercises the actual production wiring end to end (route match ->
// CaseDetailRouteSync -> CaseTabsContext -> CaseTabsContentHost's
// CaseTabIsolatedRouter), not a hand-rolled substitute for it.
vi.mock("@features/case-tabs/lazyCaseDetailPage", () => ({
  default: function StubCaseDetailPage() {
    const override = useCaseRouteOverride();
    const { caseId: routedCaseId } = useParams();
    const caseId = override?.caseId ?? routedCaseId;
    return <div data-testid="stub-page-case-id">{caseId}</div>;
  },
}));

/**
 * End-to-end smoke test for the real `App.tsx`/`AppLayout` wiring shape:
 * one real `<BrowserRouter>` (as the app always has exactly one), the same
 * case-detail route pattern App.tsx registers, `CaseTabsContentHost`
 * (App.tsx and AppLayout render this in the app shell), and
 * `CaseDetailRouteSync` as the route's own element (App.tsx's actual
 * `element={<CaseDetailRouteSync kind="case" />}`).
 *
 * This is the regression test for the "You cannot render a <Router> inside
 * another <Router>" crash a previous version of this feature shipped with:
 * that bug was only reachable through this exact real-router nesting, which
 * a standalone component render (this feature's other test files) cannot
 * reproduce.
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
});
