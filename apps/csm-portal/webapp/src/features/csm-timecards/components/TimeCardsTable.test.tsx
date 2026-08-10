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
import TimeCardsTable from "@features/csm-timecards/components/TimeCardsTable";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

const CARD: CsmTimeCard = {
  id: "tc-1",
  caseId: "case-1",
  caseNumber: "CS0352584",
  projectId: "proj-1",
  projectName: "Acme Project",
  workDate: "2026-07-01",
  userId: "user-1",
  userName: "Jane Doe",
  state: "submitted",
  billable: true,
  totalMinutes: 30,
};

const APPROVED_CARD: CsmTimeCard = {
  ...CARD,
  id: "tc-2",
  caseNumber: "CS0352585",
  userName: "John Roe",
  state: "approved",
};

const ROLE_CTX = { isOwner: false, isApprover: false, isAdmin: false };
const APPROVER_ROLE_CTX = { isOwner: false, isApprover: true, isAdmin: false };

describe("TimeCardsTable column visibility", () => {
  it("shows the Case column but not the Engineer column on the personal view (My time sheets)", () => {
    render(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.getByRole("columnheader", { name: "Case" })).toBeInTheDocument();
    expect(
      screen.queryByRole("columnheader", { name: "Engineer" }),
    ).not.toBeInTheDocument();

    expect(screen.getByText("CS0352584")).toBeInTheDocument();
    expect(screen.queryByText("Jane Doe")).not.toBeInTheDocument();
  });

  it("shows both the Case and Engineer columns when showEngineerColumn is set (All / Approvals)", () => {
    render(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        showEngineerColumn
        roleFor={() => ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.getByRole("columnheader", { name: "Case" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Engineer" })).toBeInTheDocument();

    expect(screen.getByText("CS0352584")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });
});

describe("TimeCardsTable bulk-select (Approvals tab)", () => {
  it("renders no checkboxes at all when selectable is omitted (default, every other tab)", () => {
    render(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
  });

  it("only shows a row checkbox for cards the viewer can actually approve, not e.g. an already-approved one", () => {
    render(
      <TimeCardsTable
        cards={[CARD, APPROVED_CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        selectable
        selectedIds={new Set()}
        onToggleSelect={vi.fn()}
        onToggleSelectAll={vi.fn()}
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    // One header select-all checkbox + one row checkbox (the submitted card
    // only) — the approved card gets no checkbox at all.
    expect(screen.getAllByRole("checkbox")).toHaveLength(2);
    expect(
      screen.getByRole("checkbox", { name: /select time card cs0352584/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: /select time card cs0352585/i }),
    ).not.toBeInTheDocument();
  });

  it("calls onToggleSelect with the card when its row checkbox is clicked", () => {
    const onToggleSelect = vi.fn();
    render(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        selectable
        selectedIds={new Set()}
        onToggleSelect={onToggleSelect}
        onToggleSelectAll={vi.fn()}
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: /select time card cs0352584/i }));
    expect(onToggleSelect).toHaveBeenCalledWith(CARD);
  });

  it("reflects selectedIds on the row checkbox and shows the header checkbox checked once every selectable row is selected", () => {
    render(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        selectable
        selectedIds={new Set([CARD.id])}
        onToggleSelect={vi.fn()}
        onToggleSelectAll={vi.fn()}
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    const rowCheckbox = screen.getByRole("checkbox", { name: /select time card cs0352584/i });
    const headerCheckbox = screen.getByRole("checkbox", { name: /select all approvable/i });
    expect(rowCheckbox).toBeChecked();
    expect(headerCheckbox).toBeChecked();
  });

  it("calls onToggleSelectAll with only the selectable cards on the page when the header checkbox is clicked", () => {
    const onToggleSelectAll = vi.fn();
    render(
      <TimeCardsTable
        cards={[CARD, APPROVED_CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        selectable
        selectedIds={new Set()}
        onToggleSelect={vi.fn()}
        onToggleSelectAll={onToggleSelectAll}
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: /select all approvable/i }));
    expect(onToggleSelectAll).toHaveBeenCalledWith([CARD]);
  });

  it("disables the header select-all checkbox when no row on the page is approvable", () => {
    render(
      <TimeCardsTable
        cards={[APPROVED_CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        selectable
        selectedIds={new Set()}
        onToggleSelect={vi.fn()}
        onToggleSelectAll={vi.fn()}
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.getByRole("checkbox", { name: /select all approvable/i })).toBeDisabled();
  });
});

describe("TimeCardsTable edit action", () => {
  it("shows the edit icon for the card's own owner on a submitted card, and calls onCardAction with \"edit\"", () => {
    const onCardAction = vi.fn();
    render(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => ({ isOwner: true, isApprover: false, isAdmin: false })}
        onCardAction={onCardAction}
      />,
    );

    const editButton = screen.getByTestId(`timecard-edit-${CARD.id}`);
    expect(editButton).toBeInTheDocument();
    fireEvent.click(editButton);
    expect(onCardAction).toHaveBeenCalledWith(CARD, "edit");
  });

  it("shows the edit icon regardless of showActionsColumn (unlike approve/reject)", () => {
    render(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        showActionsColumn={false}
        roleFor={() => ({ isOwner: true, isApprover: false, isAdmin: false })}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.getByTestId(`timecard-edit-${CARD.id}`)).toBeInTheDocument();
  });

  it("hides the edit icon for a non-owner (e.g. an approver viewing someone else's card)", () => {
    render(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        showActionsColumn
        roleFor={() => ({ isOwner: false, isApprover: true, isAdmin: false })}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.queryByTestId(`timecard-edit-${CARD.id}`)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Approve" })).toBeInTheDocument();
  });

  it("hides the edit icon once the card is no longer submitted, even for the owner", () => {
    render(
      <TimeCardsTable
        cards={[{ ...CARD, state: "approved" }]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => ({ isOwner: true, isApprover: false, isAdmin: false })}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.queryByTestId(`timecard-edit-${CARD.id}`)).not.toBeInTheDocument();
  });
});
