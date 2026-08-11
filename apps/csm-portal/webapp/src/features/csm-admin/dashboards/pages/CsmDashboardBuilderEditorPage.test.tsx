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
import { MemoryRouter, Route, Routes } from "react-router";

const getMock = vi.fn();
const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock, post: postMock }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));

// A window.prompt-driven flow ("Add section") — stubbed so its own tests can
// control what the admin "typed".
const promptMock = vi.fn();
vi.stubGlobal("prompt", promptMock);

import CsmDashboardBuilderEditorPage from "@features/csm-admin/dashboards/pages/CsmDashboardBuilderEditorPage";
import {
  getDashboardDraft,
  saveDashboardDraft,
} from "@features/csm-admin/dashboards/utils/dashboardDraftsStorage";

function renderEditor(initialPath: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/admin/dashboards/new" element={<CsmDashboardBuilderEditorPage />} />
          <Route path="/admin/dashboards/:draftId" element={<CsmDashboardBuilderEditorPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmDashboardBuilderEditorPage", () => {
  beforeEach(() => {
    getMock.mockReset();
    postMock.mockReset();
    // Every widget in these tests renders through the real
    // `DashboardWidgetTile` (see this page's own doc comment on reusing the
    // live render path) — give its own `/cases/search` call a real
    // response so a widget's tile doesn't sit in its own loading skeleton
    // forever (a count-shape tile only renders its `displayName` once
    // loaded — see `DashboardWidgetTile.tsx`).
    postMock.mockResolvedValue({ total: 0, cases: [], limit: 1, offset: 0, hasMore: false });
    promptMock.mockReset();
    localStorage.clear();
  });

  it("creating a new dashboard: seeds an empty draft, lets the admin name it, add a widget, and persists to localStorage", async () => {
    getMock.mockResolvedValue(null); // GET /dashboards/{newId} 404s -> null
    renderEditor("/admin/dashboards/new");

    // Seeded and rendered (not stuck on the loading skeleton).
    await waitFor(() => expect(screen.getByLabelText("Dashboard display name")).toBeInTheDocument());
    expect(
      screen.getByText(/this dashboard has never been deployed/i),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Dashboard display name"), {
      target: { value: "My new dashboard" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Add widget" }));
    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "Open cases" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add widget" }));

    await waitFor(() => expect(screen.getByText("Open cases")).toBeInTheDocument());

    // Debounced persistence — wait past the 300ms window.
    await waitFor(
      () => {
        const drafts = Object.values(localStorage)
          .map((v) => v)
          .join("");
        expect(drafts).toContain("My new dashboard");
      },
      { timeout: 1000 },
    );
  });

  it("editing an existing dashboard: seeds from GET /dashboards/{id} and shows no drift banner when unchanged", async () => {
    getMock.mockResolvedValue({
      id: "agents_pilot",
      displayName: "Engineer overview",
      isDefault: true,
      isTeamBased: false,
      widgets: [
        {
          widgetId: "w1",
          displayName: "My Patches",
          resourceType: "case",
          shape: "count",
          gridWidth: 3,
          query: {},
        },
      ],
    });

    renderEditor("/admin/dashboards/agents_pilot");

    await waitFor(() => expect(screen.getByText("My Patches")).toBeInTheDocument());
    expect(screen.queryByText(/not yet deployed/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/differs from what/i)).not.toBeInTheDocument();
  });

  it("shows the drift warning once a deployed dashboard's draft is edited locally", async () => {
    getMock.mockResolvedValue({
      id: "agents_pilot",
      displayName: "Engineer overview",
      isDefault: true,
      isTeamBased: false,
      widgets: [],
    });

    renderEditor("/admin/dashboards/agents_pilot");
    await waitFor(() => expect(screen.getByLabelText("Dashboard display name")).toHaveValue("Engineer overview"));
    expect(screen.queryByText(/not yet deployed/i)).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Dashboard display name"), {
      target: { value: "Engineer overview (renamed)" },
    });

    await waitFor(() => expect(screen.getByText(/not yet deployed/i)).toBeInTheDocument());
  });

  it("removing a widget asks for confirmation before it disappears from the draft", async () => {
    const draft = {
      id: "with-widget",
      sourceDashboardId: "with-widget",
      displayName: "Has a widget",
      isDefault: false,
      isTeamBased: false,
      widgets: [
        {
          widgetId: "w1",
          displayName: "My Patches",
          resourceType: "case" as const,
          shape: "count" as const,
          gridWidth: 3,
          query: {},
        },
      ],
      emptySections: [],
    };
    saveDashboardDraft(draft);
    getMock.mockResolvedValue({ ...draft, id: "with-widget" });

    renderEditor("/admin/dashboards/with-widget");
    await waitFor(() => expect(screen.getByText("My Patches")).toBeInTheDocument());

    // Every button query below that follows a Dialog open/close is wrapped
    // in `waitFor`: MUI's Modal keeps the rest of the page `aria-hidden`
    // until its own exit transition finishes (a real, un-mocked ~225ms
    // Fade), and `getByRole` — unlike `getByText` — respects that, so a
    // synchronous re-query right after a close click can transiently miss
    // an element that is genuinely about to reappear.
    const removeButton = await waitFor(() =>
      screen.getByRole("button", { name: "Remove widget My Patches" }),
    );
    fireEvent.click(removeButton);
    expect(screen.getByText(/removes "My Patches" from this draft/i)).toBeInTheDocument();

    // Cancelling keeps the widget.
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.getByText("My Patches")).toBeInTheDocument();

    const removeButtonAgain = await waitFor(() =>
      screen.getByRole("button", { name: "Remove widget My Patches" }),
    );
    fireEvent.click(removeButtonAgain);
    const confirmButton = await waitFor(() => screen.getByRole("button", { name: "Remove" }));
    fireEvent.click(confirmButton);

    await waitFor(() => expect(screen.queryByText("My Patches")).not.toBeInTheDocument());
    await waitFor(() => expect(getDashboardDraft("with-widget")?.widgets).toHaveLength(0));
  });

  it("adds an empty section shell via 'Add section', offering its own 'Add widget' entry point", async () => {
    getMock.mockResolvedValue(null);
    promptMock.mockReturnValue("New section");

    renderEditor("/admin/dashboards/new");
    await waitFor(() => expect(screen.getByLabelText("Dashboard display name")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Add section" }));

    expect(await screen.findByText("New section")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove section New section" })).toBeInTheDocument();
  });
});
