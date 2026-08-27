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

/** Same as {@link renderAt}, but also hands back the `QueryClient` so a
 * test can force a real background refetch of a shared cache entry (e.g.
 * `useSearchTags("", ...)`'s), the same way another mounted "Tags" control
 * elsewhere in the app would once its own `staleTime` elapses. */
function renderAtWithQueryClient(initialEntry: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const result = render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/dashboard" element={<div>Dashboard landing</div>} />
          <Route path="/dashboard/preview/:previewSlug" element={<DashboardWidgetPreviewPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { ...result, queryClient };
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
 * Regression (digiops-cs#2880): a case-family widget carrying `anyOf`
 * (cross-field OR branches) must NOT fall into `CaseFamilyWidgetPreview` —
 * `CasesFilters`/`CasesFilterBar` have no OR construct, so seeding them from
 * `translateCaseDashboardFilters` would silently drop `anyOf` and land on a
 * broader, unfiltered-by-`anyOf` result set than the tile it was reached
 * from actually counted. It falls through to the generic, filter-faithful
 * `useWidgetData`-backed content instead (same path `resourceType: "incident"`
 * always used), which posts the widget's raw filters — `anyOf` included —
 * straight to `/cases/search`.
 */
describe("DashboardWidgetPreviewPage — a case-family widget with anyOf skips the editable filter bar", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("posts the widget's anyOf branches verbatim to /cases/search instead of seeding CasesFilterBar", async () => {
    postMock.mockResolvedValue({
      cases: [{ id: "c1", number: "CS-1", subject: "Disk full", state: "open" }],
      total: 1,
      limit: 10,
      offset: 0,
    });

    const anyOf = [
      { filters: [{ field: "severity", op: "in", values: ["catastrophic", "critical"] }] },
      { filters: [{ field: "type", op: "in", values: ["security_report_analysis"] }] },
    ];

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "wow_p0p1",
        displayName: "WOW P0/P1",
        filters: {
          filters: [{ field: "state", op: "in", values: ["open"] }],
          anyOf,
        },
      }),
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    // The real, editable Cases filter bar never mounts for this widget.
    expect(screen.queryByRole("combobox", { name: "Severity" })).not.toBeInTheDocument();

    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      expect.objectContaining({
        filters: expect.objectContaining({
          filters: [{ field: "state", op: "in", values: ["open"] }],
          anyOf,
        }),
      }),
      { signal: expect.any(AbortSignal) },
    );
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

  // The core regression (this is the fix under test): a tag has no small,
  // fixed universe of values (see `CaseFamilyWidgetPreview`'s own doc
  // comment), so a widget's `tag notIn ["s_dip"]` can't be represented by a
  // real `excludeTags` control on `CasesFilterBar` (there isn't one here) --
  // it's *shown* in the single "Tags" control as the complement over the
  // currently-known tag catalog. But what's actually sent to
  // `/cases/search` must stay the real `notIn` blacklist (`apiFilters`),
  // never that catalog-derived approximation, or a case with no tags at all
  // would be wrongly excluded and the preview page would disagree with the
  // dashboard tile it was reached from.
  it("queries /cases/search with the real tag notIn blacklist, even though the 'Tags' control displays the catalog complement", async () => {
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

    // The displayed "Tags" control shows the complement (checked "other-tag").
    await waitFor(() => {
      expect(screen.getByText("other-tag")).toBeInTheDocument();
    });

    // But the query itself carries the real `notIn` blacklist, not an `in`
    // whitelist derived from the (necessarily incomplete) tag catalog.
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

    const lastCasesCall = postMock.mock.calls.filter((c) => c[0] === "/cases/search").at(-1);
    const tagEntries = (
      lastCasesCall?.[1] as { filters: { filters: { field: string; op: string }[] } }
    ).filters.filters.filter((f) => f.field === "tag");
    // No complement-derived `in` entry ever reaches the API call.
    expect(tagEntries.some((f) => f.op === "in")).toBe(false);
  });

  // Once the viewer edits the filter bar themselves, displayed and queried
  // filters must collapse to the same value -- they're now consciously
  // picking specific tags to include, so the approximation/blacklist
  // divergence no longer applies.
  it("queries exactly what's displayed once the viewer edits the filter bar themselves", async () => {
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
      expect(screen.getByRole("combobox", { name: "Tags" })).toBeInTheDocument(),
    );
    const callsBefore = postMock.mock.calls.filter((c) => c[0] === "/cases/search").length;

    fireEvent.change(screen.getByPlaceholderText(/search by case #/i), {
      target: { value: "disk" },
    });

    await waitFor(() => {
      const callsAfter = postMock.mock.calls.filter((c) => c[0] === "/cases/search").length;
      expect(callsAfter).toBeGreaterThan(callsBefore);
    });
    const lastCasesCall = postMock.mock.calls.filter((c) => c[0] === "/cases/search").at(-1);
    // The complement-derived `tag in [other-tag]` the bar still displays now
    // matches what's queried -- no more real `notIn` sent underneath it.
    expect(lastCasesCall?.[1]).toMatchObject({
      filters: expect.objectContaining({
        searchQuery: "disk",
        filters: expect.arrayContaining([
          { field: "tag", op: "in", values: ["other-tag"] },
        ]),
      }),
    });
    const tagEntries = (
      lastCasesCall?.[1] as { filters: { filters: { field: string; op: string }[] } }
    ).filters.filters.filter((f) => f.field === "tag");
    expect(tagEntries.some((f) => f.op === "notIn")).toBe(false);
  });

  // CodeRabbit finding: overwriting `tags` with the complement would
  // silently drop a widget's own "must have one of these tags" requirement
  // when it also excludes a tag. Intersecting instead preserves both -- this
  // still governs what's *displayed*; the query itself uses the real,
  // un-intersected `tag in`/`tag notIn` pair straight from the widget.
  it("intersects an existing tag-in list with the complement for display, while the query keeps the widget's real tag in/notIn pair", async () => {
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
      expect(screen.getByText("required-tag")).toBeInTheDocument();
    });

    await waitFor(() => {
      const lastCasesCall = postMock.mock.calls
        .filter((c) => c[0] === "/cases/search")
        .at(-1);
      expect(lastCasesCall).toBeDefined();
      expect(lastCasesCall?.[1]).toMatchObject({
        filters: {
          filters: expect.arrayContaining([
            { field: "tag", op: "in", values: ["required-tag"] },
            { field: "tag", op: "notIn", values: ["s_dip"] },
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

  // CodeRabbit finding: `useSearchTags("", ...)`'s cache entry is shared,
  // by query key, with every other "Tags" control mounted anywhere in the
  // app (e.g. the real one inside this very `CasesFilterBar`, or another
  // preview page open in a different tab) -- once its own `staleTime`
  // elapses, any of those refetches the exact same cache entry, which
  // flips `isFetching` back to true here too, even though this page never
  // asked for it. Reset must still restore the widget's own starting
  // filters through that window, not fall back to `DEFAULT_CASES_FILTERS`
  // just because the shared query happens to be mid-refetch at that exact
  // moment.
  it("Reset still restores the widget's own filters during a later background refetch of the shared tag-catalog query", async () => {
    // The initial catalog fetch resolves normally; a *second* call (the
    // simulated background refetch below) is held open on a promise this
    // test controls, so `isFetching` on the shared query stays true for as
    // long as needed instead of racing a click against a promise that
    // resolves in the same microtask it was created in.
    let tagsCallCount = 0;
    let resolveSecondTagsCall: (value: unknown) => void = () => {};
    postMock.mockImplementation((url: string) => {
      if (url === "/tags/search") {
        tagsCallCount += 1;
        if (tagsCallCount === 1) {
          return Promise.resolve({
            tags: [{ id: "t1", label: "s_dip" }, { id: "t2", label: "other-tag" }],
          });
        }
        return new Promise((resolve) => {
          resolveSecondTagsCall = resolve;
        });
      }
      if (url === "/teams/search") return Promise.resolve({ teams: [] });
      if (url === "/cases/search") {
        return Promise.resolve({ cases: [], total: 0, limit: 10, offset: 0 });
      }
      return Promise.resolve({});
    });

    const { queryClient } = renderAtWithQueryClient(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "excl_tag_widget",
        displayName: "Discussions on Going",
        filters: {
          filters: [
            { field: "state", op: "in", values: ["open"] },
            { field: "tag", op: "notIn", values: ["s_dip"] },
          ],
        },
      }),
    );

    // Let the initial catalog fetch (and the /cases/search it feeds) settle
    // to the widget's own resolved starting point.
    await waitFor(() => {
      const lastCasesCall = postMock.mock.calls.filter((c) => c[0] === "/cases/search").at(-1);
      expect(lastCasesCall).toBeDefined();
      expect(lastCasesCall?.[1]).toMatchObject({
        filters: {
          filters: expect.arrayContaining([{ field: "state", op: "in", values: ["open"] }]),
        },
      });
    });

    // Simulate another "Tags" control elsewhere refetching the exact same
    // shared cache entry -- not an action this page itself takes. Held open
    // by `resolveSecondTagsCall` above, so `isFetching` on
    // `["csm-tags-search", ""]` stays true through the click below rather
    // than flipping back before this test can observe it.
    void queryClient.refetchQueries({ queryKey: ["csm-tags-search", ""] });
    await waitFor(() => {
      expect(queryClient.getQueryState(["csm-tags-search", ""])?.fetchStatus).toBe("fetching");
    });

    fireEvent.click(screen.getByRole("button", { name: /clear filters/i }));
    resolveSecondTagsCall({ tags: [{ id: "t1", label: "s_dip" }, { id: "t2", label: "other-tag" }] });

    await waitFor(() => {
      const lastCasesCall = postMock.mock.calls.filter((c) => c[0] === "/cases/search").at(-1);
      expect(lastCasesCall?.[1]).toMatchObject({
        filters: {
          filters: expect.arrayContaining([{ field: "state", op: "in", values: ["open"] }]),
        },
      });
    });
  });
});
