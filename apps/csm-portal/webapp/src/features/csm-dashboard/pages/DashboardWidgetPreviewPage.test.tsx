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
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter, Route, Routes } from "react-router";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
// Pulls in widgetListConfig.tsx -> useTimeSheets.ts (time_card's mapper),
// which reads `window.config` at load via `@config/apiConfig` — same
// workaround as DashboardWidgetTile.test.tsx.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
const CURRENT_USER_ID = "11111111-aaaa-bbbb-cccc-000000000001";
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { id: CURRENT_USER_ID },
    isLoading: false,
    isError: false,
  }),
}));
// `useGetCsmCases` (pulled in transitively once a case-family widget renders
// the real CasesFilterBar/CasesList) reads `useLogger`, which needs a
// `LoggerProvider` this test doesn't otherwise set up.
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "agent@wso2.com" }),
}));

import DashboardWidgetPreviewPage from "@features/csm-dashboard/pages/DashboardWidgetPreviewPage";
import { buildWidgetPreviewHref } from "@features/csm-dashboard/utils/widgetPreviewUrl";

function renderAt(initialEntry: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/dashboard" element={<div>Dashboard landing</div>} />
          <Route path="/dashboard/preview/:previewSlug" element={<DashboardWidgetPreviewPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

/** Routes `postMock` by URL, since a case-family widget's preview page
 * fires up to three different POST endpoints at once: `/teams/search`
 * (`CasesFilterBar`'s "Team" control loads unconditionally on mount),
 * `/tags/search` (only when the widget's own filter needs the tag
 * complement — see `DashboardWidgetPreviewPage.tsx`), and `/cases/search`
 * itself. */
function mockPost(responses: {
  teams?: unknown;
  tags?: unknown;
  cases?: unknown;
}): void {
  postMock.mockImplementation((url: string) => {
    if (url === "/teams/search") return Promise.resolve(responses.teams ?? { teams: [] });
    if (url === "/tags/search") return Promise.resolve(responses.tags ?? { tags: [] });
    if (url === "/cases/search") {
      return Promise.resolve(
        responses.cases ?? { cases: [], total: 0, limit: 10, offset: 0 },
      );
    }
    return Promise.resolve({});
  });
}

describe("DashboardWidgetPreviewPage", () => {
  beforeEach(() => {
    postMock.mockReset();
    mockPost({});
  });

  it("prompts to open from a widget's View more link when the URL carries no widget params", () => {
    renderAt("/dashboard/preview/cases");
    expect(
      screen.getByText(/open this page from a dashboard widget/i),
    ).toBeInTheDocument();
  });

  it("falls back to the prompt for an unrecognized previewSlug", () => {
    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "not-a-real-resource",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Cases",
        filters: {},
      }),
    );
    expect(
      screen.getByText(/open this page from a dashboard widget/i),
    ).toBeInTheDocument();
  });

  it("returns to the dashboard when Back is clicked", () => {
    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Cases",
        filters: {},
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: /back/i }));
    expect(screen.getByText("Dashboard landing")).toBeInTheDocument();
  });
});

/**
 * `resourceType: "incident"` never routes through `CaseFamilyWidgetPreview`
 * (see `CASE_FAMILY_RESOURCE_TYPES` in `DashboardWidgetPreviewPage.tsx`), so
 * this generic `useWidgetData` + `WIDGET_LIST_RENDERERS` + "Filtered by:"
 * chip-summary path stays exactly as it always has. These were originally
 * written against `previewSlug: "cases"` fixtures purely as a convenient
 * generic resourceType — moved to "incidents" once "cases" gained its own,
 * behaviorally different branch (see the describe block below).
 */
describe("DashboardWidgetPreviewPage — generic resourceTypes keep the read-only summary + search box", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("renders the widget's table and paginates using the URL-provided widget id/filters", async () => {
    postMock.mockResolvedValue({
      total: 12,
      incidents: [{ id: "11111111-1111-1111-1111-111111111111", number: "INC-1", subject: "Disk full" }],
      limit: 10,
      offset: 0,
      hasMore: true,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "incidents",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Incidents",
        filters: { priorities: ["critical"] },
      }),
    );

    expect(screen.getByText("My Critical & High Incidents")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("INC-1")).toBeInTheDocument());
    expect(postMock).toHaveBeenCalledWith(
      "/incidents/search",
      {
        filters: { priorities: ["critical"] },
        pagination: { offset: 0, limit: 10 },
      },
      { signal: expect.any(AbortSignal) },
    );

    // TablePagination's "next page" button.
    fireEvent.click(screen.getByRole("button", { name: /next page/i }));
    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/incidents/search",
        {
          filters: { priorities: ["critical"] },
          pagination: { offset: 10, limit: 10 },
        },
        { signal: expect.any(AbortSignal) },
      ),
    );
  });

  it("resolves the masked @me sentinel back to the signed-in user's own id before querying", async () => {
    postMock.mockResolvedValue({
      total: 1,
      incidents: [{ id: "11111111-1111-1111-1111-111111111111", number: "INC-1", subject: "Disk full" }],
      limit: 10,
      offset: 0,
      hasMore: false,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "incidents",
        widgetId: "my_incidents",
        displayName: "My Incidents",
        filters: { assignedUserIds: [CURRENT_USER_ID] },
        currentUserId: CURRENT_USER_ID,
      }),
    );

    await waitFor(() => expect(screen.getByText("INC-1")).toBeInTheDocument());
    expect(postMock).toHaveBeenCalledWith(
      "/incidents/search",
      {
        filters: { assignedUserIds: [CURRENT_USER_ID] },
        pagination: { offset: 0, limit: 10 },
      },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("merges a typed search term into the widget's own filters as searchQuery", async () => {
    postMock.mockResolvedValue({
      total: 1,
      incidents: [{ id: "11111111-1111-1111-1111-111111111111", number: "INC-1", subject: "Disk full" }],
      limit: 10,
      offset: 0,
      hasMore: false,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "incidents",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Incidents",
        filters: { priorities: ["critical"] },
      }),
    );
    await waitFor(() => expect(screen.getByText("INC-1")).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText("Search"), { target: { value: "disk" } });

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/incidents/search",
        {
          filters: { priorities: ["critical"], searchQuery: "disk" },
          pagination: { offset: 0, limit: 10 },
        },
        { signal: expect.any(AbortSignal) },
      ),
    );
  });

  it("renders a visible summary of the active filter criteria (flat filter shape)", async () => {
    postMock.mockResolvedValue({
      total: 1,
      incidents: [{ id: "11111111-1111-1111-1111-111111111111", number: "INC-1", subject: "Disk full" }],
      limit: 10,
      offset: 0,
      hasMore: false,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "incidents",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Incidents",
        filters: { priorities: ["critical", "high"] },
      }),
    );

    await waitFor(() => expect(screen.getByText("INC-1")).toBeInTheDocument());
    const group = screen.getByRole("group", { name: "Active filters" });
    expect(group).toHaveTextContent("priorities: critical, high");
  });

  it("does not render an active-filters summary when the widget has no filters", async () => {
    postMock.mockResolvedValue({
      total: 1,
      incidents: [{ id: "11111111-1111-1111-1111-111111111111", number: "INC-1", subject: "Disk full" }],
      limit: 10,
      offset: 0,
      hasMore: false,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "incidents",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Incidents",
        filters: {},
      }),
    );

    await waitFor(() => expect(screen.getByText("INC-1")).toBeInTheDocument());
    expect(screen.queryByRole("group", { name: "Active filters" })).not.toBeInTheDocument();
  });
});

/**
 * Reported live: a case-family widget's "View more" landed on a static
 * "Filtered by:" chip summary, unlike every other list page in the app,
 * which has a real, editable filter bar. `CaseFamilyWidgetPreview` (in
 * `DashboardWidgetPreviewPage.tsx`) fixes this by seeding the actual
 * `CasesFilterBar` + `useGetCsmCases` + `CasesList` from the widget's own
 * filters instead.
 */
describe("DashboardWidgetPreviewPage — case-family widgets get the real, editable Cases filter bar", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("renders the real CasesFilterBar (not the read-only chip summary), seeded from the widget's own field/op/values filters", async () => {
    mockPost({
      cases: {
        cases: [{ id: "c1", number: "CS-1", subject: "Disk full", state: "open" }],
        total: 1,
        limit: 10,
        offset: 0,
      },
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "team_open_cases",
        displayName: "Team Open Cases",
        filters: { filters: [{ field: "state", op: "in", values: ["open"] }] },
      }),
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(screen.getByRole("combobox", { name: "Severity" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "State" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Tags" })).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Active filters" })).not.toBeInTheDocument();

    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      expect.objectContaining({
        filters: expect.objectContaining({
          filters: expect.arrayContaining([
            { field: "state", op: "in", values: ["open"] },
          ]),
        }),
      }),
    );
  });

  it("re-queries /cases/search when a filter is edited in the real filter bar", async () => {
    mockPost({
      cases: {
        cases: [{ id: "c1", number: "CS-1", subject: "Disk full", state: "open" }],
        total: 1,
        limit: 10,
        offset: 0,
      },
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "team_open_cases",
        displayName: "Team Open Cases",
        filters: {},
      }),
    );
    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    const callsBefore = postMock.mock.calls.filter((c) => c[0] === "/cases/search").length;

    fireEvent.change(screen.getByPlaceholderText(/search by case #/i), {
      target: { value: "disk" },
    });

    await waitFor(() => {
      const callsAfter = postMock.mock.calls.filter((c) => c[0] === "/cases/search").length;
      expect(callsAfter).toBeGreaterThan(callsBefore);
    });
    const lastCall = postMock.mock.calls
      .filter((c) => c[0] === "/cases/search")
      .at(-1);
    expect(lastCall?.[1]).toMatchObject({ filters: { searchQuery: "disk" } });
  });

  // The core regression: a tag has no small, fixed universe of values (see
  // `CaseFamilyWidgetPreview`'s own doc comment), so a widget's
  // `tag notIn ["s_dip"]` can't be represented by a real `excludeTags`
  // control (there isn't one here) -- it's seeded into the single "Tags"
  // control as the complement over the currently-known tag catalog instead.
  it("seeds the single 'Tags' control with the complement of the tag catalog for a tag notIn widget filter", async () => {
    mockPost({
      tags: { tags: [{ id: "t1", label: "s_dip" }, { id: "t2", label: "other-tag" }] },
      cases: { cases: [], total: 0, limit: 10, offset: 0 },
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "excl_tag_widget",
        displayName: "Discussions on Going",
        filters: { filters: [{ field: "tag", op: "notIn", values: ["s_dip"] }] },
      }),
    );

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith("/tags/search", expect.anything()),
    );

    await waitFor(() => {
      const lastCasesCall = postMock.mock.calls
        .filter((c) => c[0] === "/cases/search")
        .at(-1);
      expect(lastCasesCall).toBeDefined();
      expect(lastCasesCall?.[1]).toMatchObject({
        filters: {
          filters: expect.arrayContaining([
            { field: "tag", op: "in", values: ["other-tag"] },
          ]),
        },
      });
    });

    // The excluded tag itself never appears as a selected/included value.
    const lastCasesCall = postMock.mock.calls.filter((c) => c[0] === "/cases/search").at(-1);
    const tagEntry = (
      lastCasesCall?.[1] as { filters: { filters: { field: string; values: string[] }[] } }
    ).filters.filters.find((f) => f.field === "tag");
    expect(tagEntry?.values).not.toContain("s_dip");
  });

  // CodeRabbit finding: overwriting `tags` with the complement would
  // silently drop a widget's own "must have one of these tags" requirement
  // when it also excludes a tag. Intersecting instead preserves both.
  it("intersects an existing tag-in list with the complement, rather than overwriting it, when a widget has both tag in and tag notIn", async () => {
    mockPost({
      tags: {
        tags: [
          { id: "t1", label: "s_dip" },
          { id: "t2", label: "required-tag" },
          { id: "t3", label: "other-tag" },
        ],
      },
      cases: { cases: [], total: 0, limit: 10, offset: 0 },
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "in_and_notin_widget",
        displayName: "In and NotIn Widget",
        filters: {
          filters: [
            { field: "tag", op: "in", values: ["required-tag"] },
            { field: "tag", op: "notIn", values: ["s_dip"] },
          ],
        },
      }),
    );

    await waitFor(() => {
      const lastCasesCall = postMock.mock.calls
        .filter((c) => c[0] === "/cases/search")
        .at(-1);
      expect(lastCasesCall).toBeDefined();
      expect(lastCasesCall?.[1]).toMatchObject({
        filters: {
          filters: expect.arrayContaining([
            { field: "tag", op: "in", values: ["required-tag"] },
          ]),
        },
      });
    });
  });

  // CodeRabbit finding: silently dropping the exclusion on a catalog-fetch
  // failure would broaden the search to every tag instead of failing safe.
  it("falls back to the widget's raw excludeTags (still applied, just not shown as checked) when the tag catalog fails to load", async () => {
    postMock.mockImplementation((url: string) => {
      if (url === "/tags/search") return Promise.reject(new Error("network error"));
      if (url === "/cases/search") {
        return Promise.resolve({ cases: [], total: 0, limit: 10, offset: 0 });
      }
      return Promise.resolve({ teams: [] });
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "excl_tag_catalog_fails",
        displayName: "Discussions on Going",
        filters: { filters: [{ field: "tag", op: "notIn", values: ["s_dip"] }] },
      }),
    );

    await waitFor(() =>
      expect(screen.getByText(/couldn.t load the tag catalog/i)).toBeInTheDocument(),
    );

    await waitFor(() => {
      const lastCasesCall = postMock.mock.calls
        .filter((c) => c[0] === "/cases/search")
        .at(-1);
      expect(lastCasesCall).toBeDefined();
      expect(lastCasesCall?.[1]).toMatchObject({
        filters: {
          filters: expect.arrayContaining([
            { field: "tag", op: "notIn", values: ["s_dip"] },
          ]),
        },
      });
    });
  });

  it("does not fetch the tag catalog at all when the widget has no tag exclusion", async () => {
    mockPost({ cases: { cases: [], total: 0, limit: 10, offset: 0 } });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "plain_widget",
        displayName: "Plain Widget",
        filters: { filters: [{ field: "state", op: "in", values: ["open"] }] },
      }),
    );

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith("/cases/search", expect.anything()),
    );
    expect(postMock).not.toHaveBeenCalledWith("/tags/search", expect.anything());
  });
});
