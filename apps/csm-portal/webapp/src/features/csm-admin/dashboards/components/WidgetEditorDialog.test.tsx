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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";
import type { ComponentProps } from "react";
import type { BeDashboardWidget } from "@api/backend/types";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));

import WidgetEditorDialog from "@features/csm-admin/dashboards/components/WidgetEditorDialog";

function renderDialog(props: Partial<ComponentProps<typeof WidgetEditorDialog>> = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const onSave = vi.fn();
  const onClose = vi.fn();
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <WidgetEditorDialog
          widget={undefined}
          sectionSuggestions={[]}
          onClose={onClose}
          onSave={onSave}
          {...props}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { onSave, onClose };
}

describe("WidgetEditorDialog", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("disables Add widget until a display name is entered", () => {
    renderDialog();
    expect(screen.getByRole("button", { name: "Add widget" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "My widget" },
    });
    expect(screen.getByRole("button", { name: "Add widget" })).toBeEnabled();
  });

  it("saves a new widget with the filters entered in the condition editor", () => {
    const { onSave } = renderDialog();

    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "Open cases" },
    });
    fireEvent.click(screen.getByRole("button", { name: /add filter/i }));
    fireEvent.change(screen.getByLabelText("Filter field"), { target: { value: "state" } });
    fireEvent.click(screen.getByRole("button", { name: "Add widget" }));

    expect(onSave).toHaveBeenCalledTimes(1);
    const saved = onSave.mock.calls[0][0] as BeDashboardWidget;
    expect(saved.displayName).toBe("Open cases");
    expect(saved.resourceType).toBe("case");
    expect(saved.shape).toBe("count");
    expect(saved.query).toEqual({
      filters: [{ field: "state", op: "eq", values: [] }],
    });
  });

  it("runs the in-progress config through the real widget-data resolution path when Preview is clicked", async () => {
    postMock.mockResolvedValue({ total: 7, cases: [], limit: 1, offset: 0, hasMore: false });
    renderDialog();

    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "Open cases" },
    });
    fireEvent.click(screen.getByRole("button", { name: /^preview$/i }));

    await waitFor(() => expect(screen.getByText("7")).toBeInTheDocument());
    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: {},
      pagination: { offset: 0, limit: 1 },
    });
  });

  it("shows nothing fetched until Preview is explicitly clicked", () => {
    renderDialog();
    expect(postMock).not.toHaveBeenCalled();
    expect(
      screen.getByText(/click "preview" to run this widget's current settings/i),
    ).toBeInTheDocument();
  });

  it("pre-fills the form from an existing widget when editing, and offers Delete instead of Add", () => {
    const existing: BeDashboardWidget = {
      widgetId: "w1",
      displayName: "My Patches",
      resourceType: "case",
      shape: "count",
      gridWidth: 4,
      query: { filters: [{ field: "tag", op: "in", values: ["patch"] }] },
    };
    const onDelete = vi.fn();
    renderDialog({ widget: existing, onDelete });

    expect(screen.getByDisplayValue("My Patches")).toBeInTheDocument();
    expect(screen.getByText("patch")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save widget" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Delete widget" }));
    expect(onDelete).toHaveBeenCalledTimes(1);
  });

  it("clears filter conditions when the resource type changes, rather than carrying over a shape the new endpoint won't accept", () => {
    const existing: BeDashboardWidget = {
      widgetId: "w1",
      displayName: "My Patches",
      resourceType: "case",
      shape: "count",
      gridWidth: 4,
      query: { filters: [{ field: "tag", op: "in", values: ["patch"] }] },
    };
    renderDialog({ widget: existing });

    expect(screen.getByText("patch")).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Resource type" }));
    fireEvent.click(screen.getByRole("option", { name: "incident" }));

    expect(screen.queryByText("patch")).not.toBeInTheDocument();
  });
});
