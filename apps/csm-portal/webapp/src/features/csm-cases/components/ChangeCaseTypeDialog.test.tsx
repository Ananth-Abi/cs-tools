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

/** Picks a target on step 1 and advances to the review step. */
function pickTargetAndAdvance(name: RegExp): void {
  fireEvent.click(screen.getByRole("radio", { name }));
  fireEvent.click(screen.getByRole("button", { name: /^next$/i }));
}

describe("ChangeCaseTypeDialog — step 1: pick target", () => {
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
    expect(screen.getByText(/step 1 of 2/i)).toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: /^case$/i })).not.toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /security report/i })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /^engagement$/i })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /service request/i })).toBeInTheDocument();
  });

  it("moves to step 2 on Next, without calling onSubmit", () => {
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
    pickTargetAndAdvance(/^engagement$/i);
    expect(screen.getByText(/step 2 of 2/i)).toBeInTheDocument();
    expect(screen.getByText(/transferring to/i)).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

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

describe("ChangeCaseTypeDialog — step 2: transfer into engagement", () => {
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
    pickTargetAndAdvance(/^engagement$/i);
    expect(screen.getByRole("button", { name: /transfer to engagement/i })).toBeDisabled();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /engagement type/i }));
    fireEvent.click(screen.getByRole("option", { name: /consultancy/i }));
    expect(screen.getByRole("button", { name: /transfer to engagement/i })).toBeEnabled();
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
    pickTargetAndAdvance(/^engagement$/i);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /engagement type/i }));
    fireEvent.click(screen.getByRole("option", { name: /migration/i }));
    fireEvent.click(screen.getByRole("button", { name: /transfer to engagement/i }));

    expect(onSubmit).toHaveBeenCalledWith({
      targetType: "engagement",
      engagementType: "migration",
    });
  });

  it("lists the source type's lost fields", () => {
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
    pickTargetAndAdvance(/^engagement$/i);
    expect(screen.getByText(/no longer applies/i)).toBeInTheDocument();
    expect(screen.getByText("Severity")).toBeInTheDocument();
    expect(screen.getByText("Issue type")).toBeInTheDocument();
  });

  it("returns to step 1 on Back, keeping the picked target", () => {
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
    pickTargetAndAdvance(/^engagement$/i);
    fireEvent.click(screen.getByRole("button", { name: /^back$/i }));
    expect(screen.getByText(/step 1 of 2/i)).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /^engagement$/i })).toBeChecked();
  });
});

describe("ChangeCaseTypeDialog — step 2: transfer into case", () => {
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
    pickTargetAndAdvance(/^case$/i);
    const confirmBtn = screen.getByRole("button", { name: /transfer to case/i });
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
    pickTargetAndAdvance(/^case$/i);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /severity/i }));
    fireEvent.click(screen.getByRole("option", { name: /^s2/i }));
    fireEvent.click(screen.getByRole("button", { name: /transfer to case/i }));
    expect(onSubmit).toHaveBeenCalledWith({ targetType: "case", severity: "S2" });
  });
});

describe("ChangeCaseTypeDialog — step 2: not-yet-supported targets", () => {
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
    pickTargetAndAdvance(/security report/i);
    expect(
      screen.getByRole("button", { name: /transfer to security report/i }),
    ).toBeDisabled();
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
    pickTargetAndAdvance(/service request/i);
    expect(
      screen.getByRole("button", { name: /transfer to service request/i }),
    ).toBeDisabled();
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
    pickTargetAndAdvance(/security report/i);
    expect(screen.getByText(/currently has none/i)).toBeInTheDocument();
  });
});
