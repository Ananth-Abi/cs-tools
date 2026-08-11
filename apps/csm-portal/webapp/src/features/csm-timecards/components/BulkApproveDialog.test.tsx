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
import BulkApproveDialog from "@features/csm-timecards/components/BulkApproveDialog";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

function card(overrides: Partial<CsmTimeCard> = {}): CsmTimeCard {
  return {
    id: "card-1",
    caseId: "case-1",
    caseNumber: "CS0000001",
    projectId: "proj-1",
    projectName: "Acme",
    workDate: "2026-07-13",
    userId: "user-1",
    userName: "Jane Doe",
    state: "submitted",
    billable: true,
    totalMinutes: 30,
    ...overrides,
  };
}

describe("BulkApproveDialog", () => {
  it("summarizes the card count and total minutes, without listing each card individually", () => {
    render(
      <BulkApproveDialog
        cards={[
          card({ id: "a", caseNumber: "CS0000001", userName: "Jane Doe", totalMinutes: 30 }),
          card({ id: "b", caseNumber: "CS0000002", userName: "John Roe", totalMinutes: 45 }),
        ]}
        isSubmitting={false}
        onClose={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );

    expect(screen.getByText("Approve 2 time cards?")).toBeInTheDocument();
    expect(screen.getByText("2 cards · 75 min total")).toBeInTheDocument();
    // The count is the whole point of this summary — no per-card list.
    expect(screen.queryByText("CS0000001")).not.toBeInTheDocument();
    expect(screen.queryByText("Jane Doe")).not.toBeInTheDocument();
    expect(screen.queryByText("CS0000002")).not.toBeInTheDocument();
    expect(screen.queryByText("John Roe")).not.toBeInTheDocument();
  });

  it("uses singular wording for exactly one card", () => {
    render(
      <BulkApproveDialog
        cards={[card()]}
        isSubmitting={false}
        onClose={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );

    expect(screen.getByText("Approve 1 time card?")).toBeInTheDocument();
    expect(screen.getByText("1 card · 30 min total")).toBeInTheDocument();
  });

  it("calls onConfirm when the Approve button is clicked, and onClose for Cancel", () => {
    const onConfirm = vi.fn();
    const onClose = vi.fn();
    render(
      <BulkApproveDialog
        cards={[card()]}
        isSubmitting={false}
        onClose={onClose}
        onConfirm={onConfirm}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: /cancel/i }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("disables both actions while submitting", () => {
    render(
      <BulkApproveDialog
        cards={[card()]}
        isSubmitting
        onClose={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: /cancel/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Approve" })).toBeDisabled();
  });
});
