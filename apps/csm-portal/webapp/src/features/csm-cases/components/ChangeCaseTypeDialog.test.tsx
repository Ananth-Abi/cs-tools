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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ChangeCaseTypeDialog from "@features/csm-cases/components/ChangeCaseTypeDialog";
import { useSearchCatalogs } from "@features/csm-operations/api/useSearchCatalogs";
import { useCatalogItemVariables } from "@features/csm-operations/api/useCatalogItemVariables";

// @config/apiConfig reads window.config at module load time, which doesn't
// exist under vitest — anything in the import graph that transitively pulls
// it in (here, via QueryErrorState/BackendApiError) needs it stubbed first.
// Same gotcha CsmCaseDetailPage.test.tsx already works around.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

// Service Request's field step reuses the create form's catalog hooks —
// mocked the same way EditCaseDetailsDialog.test.tsx mocks its own data
// hooks, so these tests don't need a QueryClientProvider or a real backend
// client.
vi.mock("@features/csm-operations/api/useSearchCatalogs", () => ({
  useSearchCatalogs: vi.fn(),
}));
vi.mock("@features/csm-operations/api/useCatalogItemVariables", () => ({
  useCatalogItemVariables: vi.fn(),
}));

const mockUseSearchCatalogs = vi.mocked(useSearchCatalogs);
const mockUseCatalogItemVariables = vi.mocked(useCatalogItemVariables);

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub, matching EditCaseDetailsDialog.test.tsx's own pattern
function asQueryResult(v: object): any {
  return v;
}

// Reset before every test (not just after one that overrides via
// mockImplementation) so both hooks start from a known default regardless of
// what the previous test left behind.
beforeEach(() => {
  mockUseSearchCatalogs.mockReturnValue(
    asQueryResult({
      data: [
        {
          id: "cat-1",
          name: "API Manager Support",
          catalogItems: [{ id: "item-1", name: "Request environment scaling" }],
        },
      ],
      isLoading: false,
      isError: false,
      isSuccess: true,
    }),
  );
  mockUseCatalogItemVariables.mockReturnValue(
    asQueryResult({ data: [], isLoading: false, isError: false }),
  );
});

/** Picks a target on step 1 ("pick") and advances to step 2 ("fields"). */
function pickTargetAndAdvanceToFields(name: RegExp): void {
  fireEvent.click(screen.getByRole("radio", { name }));
  fireEvent.click(screen.getByRole("button", { name: /^next$/i }));
}

/** From step 2 ("fields"), advances to step 3 ("review"), where the confirm
 * button lives. */
function advanceToReview(): void {
  fireEvent.click(screen.getByRole("button", { name: /^next$/i }));
}

describe("ChangeCaseTypeDialog — step 1: pick target", () => {
  it("offers only the other 3 types — the current type isn't a selectable option", () => {
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
    expect(screen.getByText(/step 1 of 3/i)).toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: /case/i })).not.toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /^security report$/i })).toBeEnabled();
    expect(screen.getByRole("radio", { name: /^engagement$/i })).toBeEnabled();
    expect(screen.getByRole("radio", { name: /^service request$/i })).toBeEnabled();
    expect(screen.getAllByRole("radio")).toHaveLength(3);
  });

  it("never shows 'not yet available' anywhere in the flow", () => {
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
    pickTargetAndAdvanceToFields(/^service request$/i);
    advanceToReview();
    expect(screen.queryByText(/not yet available/i)).not.toBeInTheDocument();
  });

  it("moves to step 2 (fields) on Next, without calling onSubmit", () => {
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
    pickTargetAndAdvanceToFields(/^engagement$/i);
    expect(screen.getByText(/step 2 of 3/i)).toBeInTheDocument();
    expect(screen.getByText(/what engagement needs/i)).toBeInTheDocument();
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

describe("ChangeCaseTypeDialog — step 2: retained attributes carry over disabled", () => {
  it("shows the case's current project, deployment, product, watchers, and tags as disabled dropdowns", () => {
    render(
      <ChangeCaseTypeDialog
        currentType="case"
        currentSeverity="S2"
        hasAttachments
        currentProjectName="API Manager Rollout"
        currentDeploymentName="Prod EU"
        currentProductName="API Manager 4.3"
        currentWatchers={[
          { id: "u-1", name: "Alex Doe" },
          { id: "u-2", name: "Sam Lee" },
        ]}
        currentTags={[{ id: "t-1", label: "billing" }]}
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    pickTargetAndAdvanceToFields(/^engagement$/i);

    expect(screen.getByText(/carrying over unchanged/i)).toBeInTheDocument();
    expect(screen.getByText("API Manager Rollout")).toBeInTheDocument();
    expect(screen.getByText("Prod EU")).toBeInTheDocument();
    expect(screen.getByText("API Manager 4.3")).toBeInTheDocument();
    expect(screen.getByText("Alex Doe, Sam Lee")).toBeInTheDocument();
    expect(screen.getByText("billing")).toBeInTheDocument();

    expect(screen.getByRole("combobox", { name: /^project$/i })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    expect(screen.getByRole("combobox", { name: /^watchers$/i })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });

  it("never shows a retained Severity dropdown — severity is always lost when leaving case, never carried over", () => {
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
    pickTargetAndAdvanceToFields(/^engagement$/i);
    expect(screen.queryByRole("combobox", { name: /^severity$/i })).not.toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /engagement type/i }));
    fireEvent.click(screen.getByRole("option", { name: /consultancy/i }));
    advanceToReview();
    expect(screen.getByText(/no longer applies/i)).toBeInTheDocument();
    expect(screen.getByText("Severity")).toBeInTheDocument();
  });

  it("shows placeholder text when there are no watchers or tags", () => {
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
    pickTargetAndAdvanceToFields(/^engagement$/i);
    expect(screen.getAllByText(/^none$/i)).toHaveLength(2);
  });
});

describe("ChangeCaseTypeDialog — step 2 -> 3: transfer into engagement", () => {
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
    pickTargetAndAdvanceToFields(/^engagement$/i);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /engagement type/i }));
    fireEvent.click(screen.getByRole("option", { name: /consultancy/i }));
    advanceToReview();
    expect(screen.getByText(/step 3 of 3/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /transfer to engagement/i })).toBeEnabled();
  });

  it("leaves Next disabled on the fields step if engagement type was never picked", () => {
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
    pickTargetAndAdvanceToFields(/^engagement$/i);
    expect(screen.getByRole("button", { name: /^next$/i })).toBeDisabled();
    expect(screen.getByText(/required to continue/i)).toBeInTheDocument();
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
    pickTargetAndAdvanceToFields(/^engagement$/i);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /engagement type/i }));
    fireEvent.click(screen.getByRole("option", { name: /migration/i }));
    advanceToReview();
    fireEvent.click(screen.getByRole("button", { name: /transfer to engagement/i }));

    expect(onSubmit).toHaveBeenCalledWith({
      targetType: "engagement",
      engagementType: "migration",
    });
  });

  it("lists the source type's lost fields on the review step", () => {
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
    pickTargetAndAdvanceToFields(/^engagement$/i);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /engagement type/i }));
    fireEvent.click(screen.getByRole("option", { name: /migration/i }));
    advanceToReview();
    expect(screen.getByText(/no longer applies/i)).toBeInTheDocument();
    expect(screen.getByText("Severity")).toBeInTheDocument();
    expect(screen.getByText("Issue type")).toBeInTheDocument();
  });

  it("returns to step 1 on Back from fields, keeping the picked target", () => {
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
    pickTargetAndAdvanceToFields(/^engagement$/i);
    fireEvent.click(screen.getByRole("button", { name: /^back$/i }));
    expect(screen.getByText(/step 1 of 3/i)).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /^engagement$/i })).toBeChecked();
  });

  it("returns to step 2 on Back from review, keeping the filled field", () => {
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
    pickTargetAndAdvanceToFields(/^engagement$/i);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /engagement type/i }));
    fireEvent.click(screen.getByRole("option", { name: /migration/i }));
    advanceToReview();
    fireEvent.click(screen.getByRole("button", { name: /^back$/i }));
    expect(screen.getByText(/step 2 of 3/i)).toBeInTheDocument();
    expect(screen.getByText(/migration/i)).toBeInTheDocument();
  });
});

describe("ChangeCaseTypeDialog — step 2 -> 3: transfer into case", () => {
  it("blocks confirm until both severity and issue type are picked — the backend requires both", () => {
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
    pickTargetAndAdvanceToFields(/^case$/i);
    advanceToReview();
    expect(screen.getByRole("button", { name: /transfer to case/i })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: /^back$/i }));
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /severity/i }));
    fireEvent.click(screen.getByRole("option", { name: /^s2/i }));
    advanceToReview();
    // Severity alone is still not enough.
    expect(screen.getByRole("button", { name: /transfer to case/i })).toBeDisabled();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("submits severity and issue type together in one call", () => {
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
    pickTargetAndAdvanceToFields(/^case$/i);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /severity/i }));
    fireEvent.click(screen.getByRole("option", { name: /^s2/i }));
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /issue type/i }));
    fireEvent.click(screen.getByRole("option", { name: /^error$/i }));
    advanceToReview();
    fireEvent.click(screen.getByRole("button", { name: /transfer to case/i }));
    expect(onSubmit).toHaveBeenCalledWith({
      targetType: "case",
      severity: "S2",
      issueType: "error",
    });
  });
});
