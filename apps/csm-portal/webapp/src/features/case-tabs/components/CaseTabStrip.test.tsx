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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import CaseTabStrip from "@features/case-tabs/components/CaseTabStrip";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

const TAB_1: CaseTabState = {
  id: "t1",
  caseId: "CS1",
  kind: "case",
  path: "/cases/CS1",
  label: "CS1 · First case",
  hasDraft: false,
};
const TAB_2: CaseTabState = {
  id: "t2",
  caseId: "CS2",
  kind: "case",
  path: "/cases/CS2",
  label: "CS2 · Second case",
  hasDraft: true,
};

describe("CaseTabStrip", () => {
  it("renders nothing when there are no open tabs", () => {
    const { container } = render(
      <CaseTabStrip tabs={[]} activeTabId={null} onActivate={vi.fn()} onRequestClose={vi.fn()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders one chip per open tab, highlighting the active one", () => {
    render(
      <CaseTabStrip
        tabs={[TAB_1, TAB_2]}
        activeTabId="t2"
        onActivate={vi.fn()}
        onRequestClose={vi.fn()}
      />,
    );
    const tab1 = screen.getByText("CS1 · First case");
    const tab2 = screen.getByText("CS2 · Second case");
    expect(tab1).toBeInTheDocument();
    expect(tab2.closest('[role="tab"]')).toHaveAttribute("aria-selected", "true");
    expect(tab1.closest('[role="tab"]')).toHaveAttribute("aria-selected", "false");
  });

  it("shows a Loading… placeholder (not the raw caseId/UUID) when no label has resolved yet", () => {
    render(
      <CaseTabStrip
        tabs={[{ ...TAB_1, label: undefined }]}
        activeTabId="t1"
        onActivate={vi.fn()}
        onRequestClose={vi.fn()}
      />,
    );
    expect(screen.getByText("Loading…")).toBeInTheDocument();
    expect(screen.queryByText("CS1")).not.toBeInTheDocument();
  });

  it("calls onActivate when a tab chip is clicked", () => {
    const onActivate = vi.fn();
    render(
      <CaseTabStrip
        tabs={[TAB_1, TAB_2]}
        activeTabId="t1"
        onActivate={onActivate}
        onRequestClose={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByText("CS2 · Second case"));
    expect(onActivate).toHaveBeenCalledWith("t2");
  });

  it("calls onRequestClose (not onActivate) when a tab's close button is clicked", () => {
    const onActivate = vi.fn();
    const onRequestClose = vi.fn();
    render(
      <CaseTabStrip
        tabs={[TAB_1]}
        activeTabId="t1"
        onActivate={onActivate}
        onRequestClose={onRequestClose}
      />,
    );
    // The Chip's `aria-label` lands on its outer (clickable/activate) root —
    // oxygen-ui/MUI's delete affordance is a bare `aria-hidden` svg icon with
    // its own onClick, not a separately-labelled control (same limitation as
    // this codebase's other Chip-with-onDelete usage, e.g. `PinnedTabs`) —
    // so the delete click has to target that icon directly.
    fireEvent.click(screen.getByTestId("CancelIcon"));
    expect(onRequestClose).toHaveBeenCalledWith("t1");
    expect(onActivate).not.toHaveBeenCalled();
  });
});
