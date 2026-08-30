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
import CaseTabStrip, {
  type PinnedTabProps,
} from "@features/case-tabs/components/CaseTabStrip";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

const TAB_1: CaseTabState = {
  id: "t1",
  caseId: "CS1",
  kind: "case",
  path: "/cases/CS1",
  label: "CS0001",
  internalId: "CPASUB-1",
  subject: "First case subject",
  hasDraft: false,
};
const TAB_2: CaseTabState = {
  id: "t2",
  caseId: "CS2",
  kind: "case",
  path: "/cases/CS2",
  label: "CS0002",
  internalId: "CPASUB-2",
  subject: "Second case subject",
  hasDraft: true,
};

const PINNED: PinnedTabProps = { label: "Dashboard", active: false, onClick: vi.fn() };

function noopHandlers() {
  return {
    onActivate: vi.fn(),
    onRequestClose: vi.fn(),
    onCloseAll: vi.fn(),
    onCloseOthers: vi.fn(),
  };
}

describe("CaseTabStrip", () => {
  it("renders nothing when there are no open case tabs, even with a pinned tab given", () => {
    const { container } = render(
      <CaseTabStrip tabs={[]} activeTabId={null} pinnedTab={PINNED} {...noopHandlers()} />,
    );
    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByText("Dashboard")).not.toBeInTheDocument();
  });

  it("shows the pinned tab once at least one case tab is open", () => {
    render(
      <CaseTabStrip tabs={[TAB_1]} activeTabId="t1" pinnedTab={PINNED} {...noopHandlers()} />,
    );
    expect(screen.getByText("Dashboard")).toBeInTheDocument();
    expect(screen.getByText("CS0001")).toBeInTheDocument();
  });

  it("renders one chip per open tab, highlighting the active one", () => {
    render(
      <CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t2" {...noopHandlers()} />,
    );
    const tab1 = screen.getByText("CS0001");
    const tab2 = screen.getByText("CS0002");
    expect(tab1).toBeInTheDocument();
    expect(tab2.closest('[role="tab"]')).toHaveAttribute("aria-selected", "true");
    expect(tab1.closest('[role="tab"]')).toHaveAttribute("aria-selected", "false");
  });

  it("shows a Loading… placeholder (not the raw caseId/UUID) when no label has resolved yet", () => {
    render(
      <CaseTabStrip tabs={[{ ...TAB_1, label: undefined }]} activeTabId="t1" {...noopHandlers()} />,
    );
    expect(screen.getByText("Loading…")).toBeInTheDocument();
    expect(screen.queryByText("CS1")).not.toBeInTheDocument();
  });

  it("shows the internal id + subject (not the short label) in the tab's tooltip", async () => {
    render(<CaseTabStrip tabs={[TAB_1]} activeTabId="t1" {...noopHandlers()} />);
    // Chip text itself stays the short label.
    expect(screen.getByRole("tab")).toHaveTextContent("CS0001");
    // Tooltip content (internalId + subject) only mounts in the DOM once
    // hovered — oxygen-ui/MUI's Tooltip doesn't use a native `title`
    // attribute.
    fireEvent.mouseOver(screen.getByRole("tab"));
    expect(await screen.findByText("CPASUB-1 · First case subject")).toBeInTheDocument();
  });

  it("calls onActivate when a tab chip is clicked", () => {
    const handlers = noopHandlers();
    render(<CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" {...handlers} />);
    fireEvent.click(screen.getByText("CS0002"));
    expect(handlers.onActivate).toHaveBeenCalledWith("t2");
  });

  it("calls onRequestClose (not onActivate) when a tab's close button is clicked", () => {
    const handlers = noopHandlers();
    render(<CaseTabStrip tabs={[TAB_1]} activeTabId="t1" {...handlers} />);
    // The Chip's `aria-label` lands on its outer (clickable/activate) root —
    // oxygen-ui/MUI's delete affordance is a bare `aria-hidden` svg icon with
    // its own onClick, not a separately-labelled control (same limitation as
    // this codebase's other Chip-with-onDelete usage, e.g. `PinnedTabs`) —
    // so the delete click has to target that icon directly.
    fireEvent.click(screen.getByTestId("CancelIcon"));
    expect(handlers.onRequestClose).toHaveBeenCalledWith("t1");
    expect(handlers.onActivate).not.toHaveBeenCalled();
  });

  describe("right-click context menu", () => {
    it("right-clicking a tab chip offers Close other tabs and Close all tabs", () => {
      const handlers = noopHandlers();
      render(<CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" {...handlers} />);
      fireEvent.contextMenu(screen.getByText("CS0001"));
      expect(screen.getByText("Close other tabs")).toBeInTheDocument();
      expect(screen.getByText("Close all tabs")).toBeInTheDocument();

      fireEvent.click(screen.getByText("Close other tabs"));
      expect(handlers.onCloseOthers).toHaveBeenCalledWith("t1");
      expect(handlers.onCloseAll).not.toHaveBeenCalled();
    });

    it("Close all tabs from a tab's context menu closes every tab", () => {
      const handlers = noopHandlers();
      render(<CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" {...handlers} />);
      fireEvent.contextMenu(screen.getByText("CS0002"));
      fireEvent.click(screen.getByText("Close all tabs"));
      expect(handlers.onCloseAll).toHaveBeenCalledTimes(1);
    });

    it("right-clicking empty strip space offers only Close all tabs, not Close other tabs", () => {
      const handlers = noopHandlers();
      render(<CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" {...handlers} />);
      fireEvent.contextMenu(screen.getByRole("tablist"));
      expect(screen.getByText("Close all tabs")).toBeInTheDocument();
      expect(screen.queryByText("Close other tabs")).not.toBeInTheDocument();

      fireEvent.click(screen.getByText("Close all tabs"));
      expect(handlers.onCloseAll).toHaveBeenCalledTimes(1);
    });

    it("the pinned tab is never a right-click target", () => {
      const handlers = noopHandlers();
      render(
        <CaseTabStrip tabs={[TAB_1]} activeTabId="t1" pinnedTab={PINNED} {...handlers} />,
      );
      fireEvent.contextMenu(screen.getByText("Dashboard"));
      // Bubbles to the strip's own onContextMenu (the "empty space" case),
      // since the pinned chip has none of its own — offers only Close all.
      expect(screen.getByText("Close all tabs")).toBeInTheDocument();
      expect(screen.queryByText("Close other tabs")).not.toBeInTheDocument();
    });
  });
});
