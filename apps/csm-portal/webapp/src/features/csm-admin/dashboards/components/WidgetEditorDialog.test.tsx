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

  it("threads selectedTeamGroupId/selectedTeamLabel into the Preview tile, exactly as the live dashboard grid does", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });
    renderDialog({ selectedTeamGroupId: "team-group-1", selectedTeamLabel: "Castor" });

    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "Cases — {{currentTeam}}" },
    });
    fireEvent.click(screen.getByRole("button", { name: /^preview$/i }));

    // The Preview tile resolves the widget's own `{{currentTeam}}` text
    // token using the team label passed in — before this fix, no team
    // props reached the Preview tile at all, so a team-scoped widget's
    // display name (or filters) previewed unresolved/unfiltered instead of
    // what the admin would actually see on the live dashboard.
    await waitFor(() => expect(screen.getByText("Cases — Castor")).toBeInTheDocument());
  });

  it("resolves an integrationCsTeam __current_team__ filter placeholder in Preview using the given selectedTeamGroupId", async () => {
    postMock.mockResolvedValue({ total: 0, cases: [], limit: 1, offset: 0, hasMore: false });
    renderDialog({ selectedTeamGroupId: "team-group-1", selectedTeamLabel: "Castor" });

    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "My team's cases" },
    });
    fireEvent.click(screen.getByRole("button", { name: /add filter/i }));
    fireEvent.change(screen.getByLabelText("Filter field"), {
      target: { value: "integrationCsTeam" },
    });
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Operator" }));
    fireEvent.click(screen.getByRole("option", { name: "is any of" }));
    fireEvent.change(screen.getByLabelText("Filter value"), {
      target: { value: "__current_team__" },
    });
    fireEvent.keyDown(screen.getByLabelText("Filter value"), { key: "Enter" });

    fireEvent.click(screen.getByRole("button", { name: /^preview$/i }));

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith("/cases/search", {
        filters: { filters: [{ field: "integrationCsTeam", op: "in", values: ["team-group-1"] }] },
        pagination: { offset: 0, limit: 1 },
      }),
    );
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

  it("clears configured columns when the resource type changes, since a column path is resource-specific", () => {
    const existing: BeDashboardWidget = {
      widgetId: "w1",
      displayName: "My list",
      resourceType: "case",
      shape: "list",
      gridWidth: 4,
      query: {},
      columns: [{ path: "project.key", label: "Project" }],
    };
    renderDialog({ widget: existing });

    expect(screen.getByLabelText("Column path")).toHaveValue("project.key");

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Resource type" }));
    fireEvent.click(screen.getByRole("option", { name: "incident" }));

    expect(screen.queryByLabelText("Column path")).not.toBeInTheDocument();
  });

  it("clamps Row limit to a minimum of 1, same as Grid width, rather than accepting zero/negative", () => {
    const existing: BeDashboardWidget = {
      widgetId: "w1",
      displayName: "My list",
      resourceType: "case",
      shape: "list",
      gridWidth: 4,
      query: {},
      listLimit: 5,
    };
    renderDialog({ widget: existing });

    const rowLimitInput = screen.getByLabelText(/row limit/i);
    fireEvent.change(rowLimitInput, { target: { value: "0" } });
    expect(rowLimitInput).toHaveValue(1);

    fireEvent.change(rowLimitInput, { target: { value: "-5" } });
    expect(rowLimitInput).toHaveValue(1);
  });

  it("truncates a decimal Row limit rather than saving a fractional value", () => {
    const existing: BeDashboardWidget = {
      widgetId: "w1",
      displayName: "My list",
      resourceType: "case",
      shape: "list",
      gridWidth: 4,
      query: {},
      listLimit: 5,
    };
    const { onSave } = renderDialog({ widget: existing });

    const rowLimitInput = screen.getByLabelText(/row limit/i);
    fireEvent.change(rowLimitInput, { target: { value: "7.9" } });
    expect(rowLimitInput).toHaveValue(7);

    fireEvent.click(screen.getByRole("button", { name: "Save widget" }));
    const saved = onSave.mock.calls[0][0] as BeDashboardWidget;
    expect(saved.listLimit).toBe(7);
    expect(Number.isNaN(saved.listLimit)).toBe(false);
  });

  it("hides the Columns section for count/pie shapes, and shows it for list", () => {
    renderDialog();
    // Default shape is "count".
    expect(screen.queryByRole("button", { name: /add column/i })).not.toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Shape" }));
    fireEvent.click(screen.getByRole("option", { name: "pie" }));
    expect(screen.queryByRole("button", { name: /add column/i })).not.toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Shape" }));
    fireEvent.click(screen.getByRole("option", { name: "list" }));
    expect(screen.getByRole("button", { name: /add column/i })).toBeInTheDocument();
  });

  it("adds and removes column rows", () => {
    renderDialog();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Shape" }));
    fireEvent.click(screen.getByRole("option", { name: "list" }));

    fireEvent.click(screen.getByRole("button", { name: /add column/i }));
    expect(screen.getAllByLabelText("Column path")).toHaveLength(1);

    fireEvent.click(screen.getByRole("button", { name: /add column/i }));
    expect(screen.getAllByLabelText("Column path")).toHaveLength(2);

    fireEvent.click(screen.getAllByRole("button", { name: /^Remove column/ })[0]);
    expect(screen.getAllByLabelText("Column path")).toHaveLength(1);
  });

  it("wires configured columns into buildWidget, omitting the field entirely when none are set", () => {
    const { onSave } = renderDialog();
    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "Case list" },
    });
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Shape" }));
    fireEvent.click(screen.getByRole("option", { name: "list" }));

    fireEvent.click(screen.getByRole("button", { name: "Add widget" }));
    const savedWithoutColumns = onSave.mock.calls[0][0] as BeDashboardWidget;
    expect(savedWithoutColumns.columns).toBeUndefined();
  });

  it("wires configured columns into buildWidget when rows are filled in", () => {
    const { onSave } = renderDialog();
    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "Case list" },
    });
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Shape" }));
    fireEvent.click(screen.getByRole("option", { name: "list" }));

    fireEvent.click(screen.getByRole("button", { name: /add column/i }));
    fireEvent.change(screen.getByLabelText("Column path"), {
      target: { value: "project.key" },
    });
    fireEvent.change(screen.getByLabelText("Column label"), {
      target: { value: "Project" },
    });
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Format" }));
    fireEvent.click(screen.getByRole("option", { name: "date" }));

    fireEvent.click(screen.getByRole("button", { name: "Add widget" }));
    const saved = onSave.mock.calls[0][0] as BeDashboardWidget;
    expect(saved.columns).toEqual([{ path: "project.key", label: "Project", format: "date" }]);
  });

  it("passes the configured columns to the Preview tile so it exercises the generic column renderer", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [{ id: "c-1", project: { key: "PROJ-1" } }],
      limit: 4,
      offset: 0,
      hasMore: false,
    });
    renderDialog();
    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "Case list" },
    });
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Shape" }));
    fireEvent.click(screen.getByRole("option", { name: "list" }));

    fireEvent.click(screen.getByRole("button", { name: /add column/i }));
    fireEvent.change(screen.getByLabelText("Column path"), {
      target: { value: "project.key" },
    });
    fireEvent.change(screen.getByLabelText("Column label"), {
      target: { value: "Project" },
    });

    fireEvent.click(screen.getByRole("button", { name: /^preview$/i }));

    // "Project" is this configured column's own header, only rendered by
    // `GenericColumnList`/`DashboardMiniTable` — the hardcoded per-
    // resourceType `CasesList` renderer has no such header, so seeing it
    // proves the `columns` prop actually reached the Preview tile.
    await waitFor(() => expect(screen.getByText("Project")).toBeInTheDocument());
    expect(screen.getByText("PROJ-1")).toBeInTheDocument();
  });

  it("renders the list-shape Preview at full width, without the old fixed 420px cap", async () => {
    postMock.mockResolvedValue({ total: 0, cases: [], limit: 4, offset: 0, hasMore: false });
    const { container } = render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <MemoryRouter>
          <WidgetEditorDialog widget={undefined} sectionSuggestions={[]} onClose={vi.fn()} onSave={vi.fn()} />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "Case list" },
    });
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Shape" }));
    fireEvent.click(screen.getByRole("option", { name: "list" }));

    fireEvent.click(screen.getByRole("button", { name: /^preview$/i }));

    await waitFor(() =>
      expect(screen.getAllByText("Case list").length).toBeGreaterThan(0),
    );
    // No element in the dialog carries the old fixed-width cap any more —
    // a list-shape preview now sizes to the dialog's own content width
    // instead.
    expect(container.querySelector('[style*="max-width: 420px"]')).toBeNull();
  });

  it("shows a helper hint instead of a populated dropdown before Preview has ever run", () => {
    renderDialog();
    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "Case list" },
    });
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Shape" }));
    fireEvent.click(screen.getByRole("option", { name: "list" }));

    fireEvent.click(screen.getByRole("button", { name: /add column/i }));

    expect(screen.getByText("Preview to see available fields")).toBeInTheDocument();

    // Still a plain text field — free text works even with no discovered
    // options, so column configuration is never blocked on previewing first.
    fireEvent.change(screen.getByLabelText("Column path"), {
      target: { value: "some.made.up.path" },
    });
    expect(screen.getByLabelText("Column path")).toHaveValue("some.made.up.path");
    expect(postMock).not.toHaveBeenCalled();
  });

  it("populates the Column path autocomplete with real paths discovered from Preview data, and still accepts free text", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [{ id: "c-1", project: { key: "PROJ-1", name: "Foo" } }],
      limit: 4,
      offset: 0,
      hasMore: false,
    });
    renderDialog();
    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "Case list" },
    });
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Shape" }));
    fireEvent.click(screen.getByRole("option", { name: "list" }));

    fireEvent.click(screen.getByRole("button", { name: /^preview$/i }));

    fireEvent.click(screen.getByRole("button", { name: /add column/i }));
    await waitFor(() =>
      expect(screen.queryByText("Preview to see available fields")).not.toBeInTheDocument(),
    );

    const pathInput = screen.getByLabelText("Column path");
    fireEvent.mouseDown(pathInput);
    expect(await screen.findByRole("option", { name: "project.key" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "project.name" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "id" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("option", { name: "project.key" }));
    expect(pathInput).toHaveValue("project.key");

    // Free text still works — not every real field shows up in a sampled
    // preview (e.g. null/absent on every sampled row but valid elsewhere).
    fireEvent.change(pathInput, { target: { value: "some.other.field" } });
    expect(pathInput).toHaveValue("some.other.field");
  });

  it("fetches Preview data for column-path discovery exactly once, sharing the Preview tile's own request rather than firing a second one", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [{ id: "c-1", project: { key: "PROJ-1" } }],
      limit: 4,
      offset: 0,
      hasMore: false,
    });
    renderDialog();
    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "Case list" },
    });
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Shape" }));
    fireEvent.click(screen.getByRole("option", { name: "list" }));

    fireEvent.click(screen.getByRole("button", { name: /^preview$/i }));

    fireEvent.click(screen.getByRole("button", { name: /add column/i }));
    fireEvent.mouseDown(screen.getByLabelText("Column path"));
    await screen.findByRole("option", { name: "project.key" });

    expect(postMock).toHaveBeenCalledTimes(1);
  });

  it("clearing Row limit entirely unsets it, rather than writing NaN through", () => {
    const existing: BeDashboardWidget = {
      widgetId: "w1",
      displayName: "My list",
      resourceType: "case",
      shape: "list",
      gridWidth: 4,
      query: {},
      listLimit: 5,
    };
    // `<input type="number">` sanitizes genuinely non-numeric text (verified
    // directly against jsdom's own `HTMLInputElement` — it never lets a
    // change event carry a value `Number()` would turn into `NaN`) down to
    // an empty string before a change event ever fires, so the reachable
    // "invalid input" case through this field is the empty string, which
    // this asserts resolves to `undefined` (no limit), not `NaN` (which
    // `JSON.stringify`s to `null` and would silently corrupt the deployable
    // widget JSON — see `Number.isFinite` guard in the field's own
    // `onChange`).
    const { onSave } = renderDialog({ widget: existing });

    const rowLimitInput = screen.getByLabelText(/row limit/i);
    fireEvent.change(rowLimitInput, { target: { value: "" } });
    expect(rowLimitInput).toHaveValue(null);

    fireEvent.click(screen.getByRole("button", { name: "Save widget" }));
    const saved = onSave.mock.calls[0][0] as BeDashboardWidget;
    expect(saved.listLimit).toBeUndefined();
    expect(Number.isNaN(saved.listLimit)).toBe(false);
  });
});
