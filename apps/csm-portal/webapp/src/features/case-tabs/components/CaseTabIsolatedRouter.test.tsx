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
import { describe, expect, it } from "vitest";
import { useState, type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import "@testing-library/jest-dom/vitest";
import CaseTabIsolatedRouter from "@features/case-tabs/components/CaseTabIsolatedRouter";
import { CaseTabsProvider, useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

/** Stand-in for `CsmCaseDetailPage` — a real page with local state (a text
 * input, matching `CsmCaseDetailPage`'s comment-draft state), reading its
 * `caseId` from the route exactly the same way the real page does. Keeping
 * this component independent of the (very large) real page is what lets
 * this test verify the isolated-router mechanism itself in isolation. */
function StubCasePage(): JSX.Element {
  const { caseId } = useParams();
  const [draft, setDraft] = useState("");
  const navigate = useNavigate();
  return (
    <div>
      <div data-testid="stub-case-id">{caseId}</div>
      <input
        aria-label="draft"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
      />
      <button onClick={() => navigate(`/cases/OTHER-CASE`)}>related-case-link</button>
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

describe("CaseTabIsolatedRouter", () => {
  it("resolves the correct caseId per tab via its own isolated router match", () => {
    render(
      <CaseTabsProvider>
        <Harness tabs={[TAB_A, TAB_B]} visibleId="t-a" />
      </CaseTabsProvider>,
    );
    const panels = screen.getAllByTestId("stub-case-id");
    expect(panels.map((p) => p.textContent)).toEqual(["CS1", "CS2"]);
  });

  it("hides the inactive tab's panel via CSS but keeps it mounted", () => {
    render(
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
    const { rerender } = render(
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
      <CaseTabsProvider>
        <Harness tabs={[TAB_A, TAB_B]} visibleId="t-b" />
      </CaseTabsProvider>,
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
    render(
      <CaseTabsProvider>
        <OpenerHarness />
      </CaseTabsProvider>,
    );
    fireEvent.click(screen.getByText("seed"));
    expect(screen.getByTestId("tab-count")).toHaveTextContent("1");
    fireEvent.click(screen.getByText("related-case-link"));
    expect(screen.getByTestId("tab-count")).toHaveTextContent("2");
    // The original tab (CS1) is untouched; the new one (OTHER-CASE) is now active.
    const ids = screen.getAllByTestId("stub-case-id").map((n) => n.textContent);
    expect(ids).toContain("CS1");
    expect(ids).toContain("OTHER-CASE");
  });
});
