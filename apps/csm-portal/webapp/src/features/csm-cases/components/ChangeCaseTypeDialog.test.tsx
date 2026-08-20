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
import ChangeCaseTypeDialog from "@features/csm-cases/components/ChangeCaseTypeDialog";

// Target-picker buttons are queried by data-testid rather than accessible
// name: the confirm button's own label ("Transfer to Security report")
// contains the target label as a substring, so a name-based query for e.g.
// "security report" matches both and throws on ambiguity.

describe("ChangeCaseTypeDialog — target selection", () => {
  it("offers the other 3 transferable types, not the current one", () => {
    render(
      <ChangeCaseTypeDialog
        currentType="case"
        currentSeverity="S2"
        hasAttachments
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    expect(screen.queryByTestId("transfer-target-case")).not.toBeInTheDocument();
    expect(screen.getByTestId("transfer-target-security_report_analysis")).toBeInTheDocument();
    expect(screen.getByTestId("transfer-target-engagement")).toBeInTheDocument();
    expect(screen.getByTestId("transfer-target-service_request")).toBeInTheDocument();
  });
});

describe("ChangeCaseTypeDialog — transfer into engagement", () => {
  it("requires engagement type before the transfer button enables", () => {
    render(
      <ChangeCaseTypeDialog
        currentType="case"
        currentSeverity="S2"
        hasAttachments
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fireEvent.click(screen.getByTestId("transfer-target-engagement"));
    expect(screen.getByTestId("transfer-confirm-button")).toBeDisabled();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /engagement type/i }));
    fireEvent.click(screen.getByRole("option", { name: /consultancy/i }));
    expect(screen.getByTestId("transfer-confirm-button")).toBeEnabled();
  });

  it("submits type and engagementType together", () => {
    const onSubmit = vi.fn();
    render(
      <ChangeCaseTypeDialog
        currentType="case"
        currentSeverity="S2"
        hasAttachments
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fireEvent.click(screen.getByTestId("transfer-target-engagement"));
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /engagement type/i }));
    fireEvent.click(screen.getByRole("option", { name: /migration/i }));
    fireEvent.click(screen.getByTestId("transfer-confirm-button"));

    expect(onSubmit).toHaveBeenCalledWith({
      targetType: "engagement",
      engagementType: "migration",
    });
  });

  it("lists the source type's lost fields with a reason", () => {
    render(
      <ChangeCaseTypeDialog
        currentType="case"
        currentSeverity="S2"
        hasAttachments
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fireEvent.click(screen.getByTestId("transfer-target-engagement"));
    expect(screen.getByText(/query cases are triaged/i)).toBeInTheDocument();
    expect(screen.getByText(/only classifies plain support queries/i)).toBeInTheDocument();
  });
});

describe("ChangeCaseTypeDialog — transfer into case", () => {
  it("allows transfer without picking a severity — it's optional", () => {
    const onSubmit = vi.fn();
    render(
      <ChangeCaseTypeDialog
        currentType="engagement"
        currentSeverity="unset"
        hasAttachments
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fireEvent.click(screen.getByTestId("transfer-target-case"));
    const confirmBtn = screen.getByTestId("transfer-confirm-button");
    expect(confirmBtn).toBeEnabled();
    fireEvent.click(confirmBtn);
    expect(onSubmit).toHaveBeenCalledWith({ targetType: "case", severity: undefined });
  });

  it("includes the picked severity when one is chosen", () => {
    const onSubmit = vi.fn();
    render(
      <ChangeCaseTypeDialog
        currentType="engagement"
        currentSeverity="unset"
        hasAttachments
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fireEvent.click(screen.getByTestId("transfer-target-case"));
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /severity/i }));
    fireEvent.click(screen.getByRole("option", { name: /^s2/i }));
    fireEvent.click(screen.getByTestId("transfer-confirm-button"));
    expect(onSubmit).toHaveBeenCalledWith({ targetType: "case", severity: "S2" });
  });
});

describe("ChangeCaseTypeDialog — not-yet-supported targets", () => {
  it("keeps the transfer button disabled for security_report_analysis", () => {
    render(
      <ChangeCaseTypeDialog
        currentType="case"
        currentSeverity="S2"
        hasAttachments
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fireEvent.click(screen.getByTestId("transfer-target-security_report_analysis"));
    expect(screen.getByTestId("transfer-confirm-button")).toBeDisabled();
  });

  it("keeps the transfer button disabled for service_request", () => {
    render(
      <ChangeCaseTypeDialog
        currentType="case"
        currentSeverity="S2"
        hasAttachments
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fireEvent.click(screen.getByTestId("transfer-target-service_request"));
    expect(screen.getByTestId("transfer-confirm-button")).toBeDisabled();
  });

  it("warns when the target requires an attachment the case doesn't have", () => {
    render(
      <ChangeCaseTypeDialog
        currentType="case"
        currentSeverity="S2"
        hasAttachments={false}
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fireEvent.click(screen.getByTestId("transfer-target-security_report_analysis"));
    expect(screen.getByText(/currently has none/i)).toBeInTheDocument();
  });
});

describe("ChangeCaseTypeDialog — cancel", () => {
  it("calls onClose without calling onSubmit", () => {
    const onClose = vi.fn();
    const onSubmit = vi.fn();
    render(
      <ChangeCaseTypeDialog
        currentType="case"
        currentSeverity="S2"
        hasAttachments
        isSubmitting={false}
        onClose={onClose}
        onSubmit={onSubmit}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^cancel$/i }));
    expect(onClose).toHaveBeenCalled();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
