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

import { Box, Button, Chip, TablePagination, TextField, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import type { BeWidgetResourceType } from "@api/backend/types";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useWidgetData } from "@features/csm-dashboard/api/useWidgetData";
import RefreshButton from "@components/RefreshButton";
import { WIDGET_LIST_RENDERERS } from "@features/csm-dashboard/config/widgetListConfig";
import {
  resourceTypeForPreviewSlug,
  translateCaseDashboardFilters,
} from "@features/csm-dashboard/config/widgetResourceConfig";
import {
  describeWidgetFilters,
  parseWidgetPreviewFilters,
  resolveCurrentUserSentinels,
} from "@features/csm-dashboard/utils/widgetPreviewUrl";
import CasesFilterBar, {
  type CasesFilters,
} from "@features/csm-cases/components/CasesFilterBar";
import CasesList from "@features/csm-cases/components/CasesList";
import { useGetCsmCases } from "@features/csm-cases/api/useGetCsmCases";
import { useSearchTags } from "@features/csm-cases/api/useSearchTags";
import { DEFAULT_CASES_FILTERS } from "@features/csm-cases/utils/casesFiltersUrl";

const DEFAULT_ROWS_PER_PAGE = 10;
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];
const SEARCH_DEBOUNCE_MS = 300;

/**
 * Every widget `resourceType` that ultimately queries `/cases/search` (see
 * `WIDGET_RESOURCE_CONFIG` in `widgetResourceConfig.ts` — they all share the
 * same response shape and the same `CaseWidgetList` renderer). Reported
 * live: for these, a static "Filtered by:" chip summary read wrong next to
 * every other list page in the app, which uses a real, editable filter bar
 * — so this "View more" destination stays its own dedicated, bookmarkable
 * route (that part was correct), but its filter UI is now the actual
 * `CasesFilterBar` + `useGetCsmCases` + `CasesList`, the same trio the Cases
 * tab itself uses, seeded from the widget's own filters via
 * `translateCaseDashboardFilters` and then fully editable from there.
 */
const CASE_FAMILY_RESOURCE_TYPES = new Set<BeWidgetResourceType>([
  "case",
  "service_request",
  "security_report_analysis",
  "announcement",
  "engagement",
]);

/**
 * "View more" landing for a dashboard `shape: "list"` widget tile — the same
 * per-resourceType table the tile itself renders (see `widgetListConfig.tsx`;
 * e.g. cases render through the identical `CasesList` the Cases tab uses),
 * paginated (real `TablePagination`, not just a bigger fixed fetch) so a
 * viewer can browse the widget's whole matching set from here without
 * leaving to the resource's own tab, plus a free-text search box merged
 * into the widget's own filters (`searchQuery` — the same field every other
 * resource search in this app already uses).
 *
 * Fully URL-driven (see `buildWidgetPreviewHref` in `widgetPreviewUrl.ts`)
 * rather than router-state-based, so the page is bookmarkable/shareable and
 * survives a refresh: the resource type is `:previewSlug` in the path, the
 * widget's own id/display name are `w`/`n` query params, and each filter
 * field is its own readable query param (the signed-in user's own id, where
 * present, is masked to `@me` rather than embedded verbatim). A URL with no
 * recognizable `previewSlug` or missing required params falls back to a
 * "go to the dashboard" prompt instead of crashing.
 */
export default function DashboardWidgetPreviewPage(): JSX.Element {
  const { previewSlug } = useParams<{ previewSlug: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { user, isLoading: isLoadingUser } = useCurrentUser();

  const resourceType = resourceTypeForPreviewSlug(previewSlug);
  const widgetId = searchParams.get("w");
  const displayName = searchParams.get("n");
  const { filters: rawFilters, needsCurrentUser } =
    parseWidgetPreviewFilters(searchParams);

  const backButton = (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={() => navigate("/dashboard")}
      sx={{ alignSelf: "flex-start" }}
    >
      Back
    </Button>
  );

  if (!resourceType || !widgetId || !displayName) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {backButton}
        <Typography variant="body2" color="text.secondary">
          Open this page from a dashboard widget&rsquo;s &ldquo;View
          more&rdquo; link.
        </Typography>
      </Box>
    );
  }

  // A widget filtered to "assigned to me" carries the `@me` sentinel until
  // the signed-in user's own id is known — hold off resolving/querying
  // until then rather than ever sending the literal placeholder upstream.
  if (needsCurrentUser && !user?.id) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {backButton}
        <Typography variant="h5">{displayName}</Typography>
        <Typography variant="body2" color="text.secondary">
          {isLoadingUser
            ? "Loading…"
            : "Could not resolve the signed-in user for this widget."}
        </Typography>
      </Box>
    );
  }

  const filters = resolveCurrentUserSentinels(rawFilters, user?.id);

  if (CASE_FAMILY_RESOURCE_TYPES.has(resourceType)) {
    return (
      <CaseFamilyWidgetPreview
        displayName={displayName}
        filters={filters}
        backButton={backButton}
      />
    );
  }

  return (
    <DashboardWidgetPreviewContent
      widgetId={widgetId}
      displayName={displayName}
      resourceType={resourceType}
      filters={filters}
      backButton={backButton}
    />
  );
}

interface CaseFamilyWidgetPreviewProps {
  displayName: string;
  filters: Record<string, unknown>;
  backButton: JSX.Element;
}

/**
 * The case-family "View more" landing: a real, editable `CasesFilterBar`
 * seeded from the widget's own filters (translated once via
 * `translateCaseDashboardFilters`, the same function that already builds
 * the tile's own click-through URL), feeding the actual `useGetCsmCases` +
 * `CasesList` the Cases tab itself uses — not a read-only render of
 * whatever the widget happened to be configured with.
 *
 * `CasesFilterBar` has exactly one "Tags" control, not a second "Exclude
 * tags" one (a tag has no small, fixed universe of values, unlike
 * `onboardingStatuses`, so a real, independent `excludeTags` field/control
 * still exists everywhere else `CasesFilterBar` is used — the general Cases
 * list included). Here specifically, though, a widget's own `tag notIn [X]`
 * is seeded into that single "Tags" control as its complement over the
 * currently-known tag catalog (every other tag currently on offer) rather
 * than surfacing as a separate `excludeTags` value the control can't show —
 * reported live as confusing next to a control explicitly asked to stay a
 * single multi-select. This is a deliberately approximate complement (the
 * catalog is "recently/commonly used tags", not literally every tag that
 * has ever existed), accepted as a known tradeoff rather than the exact
 * complement `onboardingStatuses` gets over its 4 fixed values.
 */
function CaseFamilyWidgetPreview({
  displayName,
  filters,
  backButton,
}: CaseFamilyWidgetPreviewProps): JSX.Element {
  // Stable across this component's lifetime (a fresh "View more" click is a
  // fresh mount) so the tag-complement effect below only ever fires once.
  const [rawTranslated] = useState(() => translateCaseDashboardFilters(filters));
  const needsTagComplement = rawTranslated.excludeTags?.length ? true : false;

  // Only the complement path needs the catalog -- an ordinary `tags`/no-tag
  // widget never issues this request at all.
  const { data: tagCatalog, isFetching: isCatalogFetching, isError: isCatalogError } =
    useSearchTags("", needsTagComplement);

  const initialCasesFilters = useMemo<CasesFilters | null>(() => {
    if (!needsTagComplement) {
      return { ...DEFAULT_CASES_FILTERS, ...rawTranslated };
    }
    // Waiting on the catalog (or it failed) -- resolved below once
    // `tagCatalog` lands; a failure falls through to no tags pre-selected
    // (broader, not narrower, than the widget's own intent) rather than
    // blocking the page forever.
    if (isCatalogFetching) return null;
    const excluded = new Set(rawTranslated.excludeTags);
    const complementTags = (tagCatalog ?? [])
      .map((t) => t.label)
      .filter((label) => !excluded.has(label));
    return {
      ...DEFAULT_CASES_FILTERS,
      ...rawTranslated,
      tags: complementTags,
      excludeTags: [],
    };
  }, [needsTagComplement, isCatalogFetching, tagCatalog, rawTranslated]);

  const [casesFilters, setCasesFilters] = useState<CasesFilters | null>(null);
  // Seeds `casesFilters` from `initialCasesFilters` exactly once, as soon as
  // it resolves (immediately for a non-tag-complement widget; after the
  // catalog fetch settles otherwise) -- never re-seeds afterward, so it
  // can't clobber an edit the viewer already made while the catalog was
  // still loading.
  if (casesFilters === null && initialCasesFilters !== null) {
    setCasesFilters(initialCasesFilters);
  }

  const [isFiltersOpen, setIsFiltersOpen] = useState(true);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const handleFiltersChange = (next: CasesFilters): void => {
    setCasesFilters(next);
    setPage(0);
  };

  const handleRowsPerPageChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  const { data, isLoading, isError, isFetching, refetch, dataUpdatedAt } = useGetCsmCases(
    casesFilters ?? DEFAULT_CASES_FILTERS,
    page,
    rowsPerPage,
    casesFilters !== null,
  );

  if (casesFilters === null) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {backButton}
        <Typography variant="h5">{displayName}</Typography>
        <Typography variant="body2" color="text.secondary">
          Loading…
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {backButton}
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="h5">{displayName}</Typography>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label={`Refresh ${displayName}`}
        />
      </Box>
      {isCatalogError && (
        <Typography variant="caption" color="text.secondary">
          Couldn&rsquo;t load the tag catalog to reflect this widget&rsquo;s tag exclusion — no
          tags are pre-selected below; pick them manually if needed.
        </Typography>
      )}
      <CasesFilterBar
        filters={casesFilters}
        onChange={handleFiltersChange}
        onReset={() => setCasesFilters(initialCasesFilters ?? DEFAULT_CASES_FILTERS)}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen((prev) => !prev)}
        availableAssigneeUsers={[]}
        availableProjects={[]}
      />
      {isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load this widget.
        </Typography>
      ) : (
        <>
          <CasesList cases={data?.cases ?? []} isLoading={isLoading} skeletonCount={rowsPerPage} />
          <TablePagination
            component="div"
            count={data?.total ?? 0}
            page={page}
            onPageChange={(_, newPage) => setPage(newPage)}
            rowsPerPage={rowsPerPage}
            onRowsPerPageChange={handleRowsPerPageChange}
            rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
            showFirstButton
            showLastButton
          />
        </>
      )}
    </Box>
  );
}

interface DashboardWidgetPreviewContentProps {
  widgetId: string;
  displayName: string;
  resourceType: BeWidgetResourceType;
  filters: Record<string, unknown>;
  backButton: JSX.Element;
}

function DashboardWidgetPreviewContent({
  widgetId,
  displayName,
  resourceType,
  filters,
  backButton,
}: DashboardWidgetPreviewContentProps): JSX.Element {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, SEARCH_DEBOUNCE_MS);

  const queriedFilters = useMemo(() => {
    const trimmed = debouncedSearch.trim();
    return trimmed ? { ...filters, searchQuery: trimmed } : filters;
  }, [filters, debouncedSearch]);

  // What's actually being queried, made visible rather than trusted
  // silently — the exact filters this page is about to send, in the same
  // already-resolved shape `useWidgetData` below queries with (no
  // `__current_team__`/`@me` placeholders left to decode). Excludes the
  // free-text search term, which the search box right below already shows.
  const filterSummary = useMemo(() => describeWidgetFilters(filters), [filters]);

  const { data, isLoading, isError, isFetching, refetch, dataUpdatedAt } = useWidgetData(
    widgetId,
    resourceType,
    queriedFilters,
    "list",
    rowsPerPage,
    page * rowsPerPage,
  );
  const ListRenderer = WIDGET_LIST_RENDERERS[resourceType];

  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setSearch(e.target.value);
    setPage(0);
  };

  const handleRowsPerPageChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {backButton}
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="h5">{displayName}</Typography>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label={`Refresh ${displayName}`}
        />
      </Box>
      {filterSummary.length > 0 && (
        <Box
          role="group"
          aria-label="Active filters"
          sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1 }}
        >
          <Typography variant="caption" color="text.secondary">
            Filtered by:
          </Typography>
          {filterSummary.map((entry) => (
            <Chip
              key={`${entry.field}-${entry.op ?? "in"}`}
              size="small"
              variant="outlined"
              label={`${entry.field}${entry.op ? ` (${entry.op})` : ""}: ${entry.value}`}
            />
          ))}
        </Box>
      )}
      <TextField
        size="small"
        label="Search"
        placeholder="Search…"
        value={search}
        onChange={handleSearchChange}
        slotProps={{ htmlInput: { "aria-label": "Search" } }}
        sx={{ maxWidth: 360 }}
      />
      {isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load this widget.
        </Typography>
      ) : (
        <>
          <ListRenderer items={data?.items ?? []} isLoading={isLoading} />
          <TablePagination
            component="div"
            count={data?.total ?? 0}
            page={page}
            onPageChange={(_, newPage) => setPage(newPage)}
            rowsPerPage={rowsPerPage}
            onRowsPerPageChange={handleRowsPerPageChange}
            rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
            showFirstButton
            showLastButton
          />
        </>
      )}
    </Box>
  );
}
