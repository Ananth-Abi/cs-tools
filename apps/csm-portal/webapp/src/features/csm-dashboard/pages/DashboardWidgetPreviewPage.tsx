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

import {
  Box,
  Button,
  Chip,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  TablePagination,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
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
  isAnyOfBranchArray,
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
import DateRangeFilter, {
  type DateRangeFilterValue,
} from "@features/csm-dashboard/components/DateRangeFilter";

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
 *
 * Exception: a widget carrying `anyOf` (cross-field OR — see
 * `isAnyOfBranchArray`) skips this editable path entirely and falls through
 * to the plain `DashboardWidgetPreviewContent` below instead (see the
 * `!isAnyOfBranchArray(...)` check at the call site) — `CasesFilters` has no
 * OR construct to seed `CasesFilterBar` with, so `translateCaseDashboardFilters`
 * would have to silently drop `anyOf` the same way `casesHref`'s own
 * click-through used to (the bug `caseFamilyBuildHref` in
 * `widgetResourceConfig.ts` exists to close). `DashboardWidgetPreviewContent`
 * posts the widget's raw, un-translated filters straight to `/cases/search`
 * via `useWidgetData` — the same request shape the tile's own count used —
 * so its result set is guaranteed to match, at the cost of a plain search box
 * instead of a fully editable filter bar for just this one case.
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

  if (CASE_FAMILY_RESOURCE_TYPES.has(resourceType) && !isAnyOfBranchArray(filters.anyOf)) {
    return (
      <CaseFamilyWidgetPreview
        displayName={displayName}
        filters={filters}
        backButton={backButton}
      />
    );
  }

  if (resourceType === "case_feedback") {
    return (
      <CaseFeedbackWidgetPreview
        widgetId={widgetId}
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
 * complement `onboardingStatuses` gets over its 4 fixed values — in
 * particular, a case with no tags at all can't be represented as "one of
 * the complement's tags" and is wrongly excluded by this approximation, a
 * gap this deliberately doesn't try to work around (doing so would mean
 * going back to a separate, non-dropdown representation of the exclusion,
 * which is exactly what was asked against). Two narrower correctness gaps
 * *are* worth guarding even within this approximation, though: a widget
 * that also has its own `tag in [...]` gets that list intersected with the
 * complement rather than overwritten (so the widget's "must have one of
 * these tags" requirement survives alongside the exclusion), and a failed
 * tag-catalog fetch falls back to the widget's raw, un-complemented
 * `excludeTags` (still correctly scoped, just not shown as checked items)
 * rather than silently dropping the exclusion and broadening the search.
 *
 * That approximation is *display-only*, though. What actually gets sent to
 * `/cases/search` is a second, independent piece of state (`apiFilters`),
 * seeded straight from `rawTranslated` -- the real `excludeTags` blacklist,
 * never the catalog-derived complement -- so a fresh load or a Reset queries
 * exactly what the dashboard tile itself queries (a case with no tags at
 * all correctly passes a `notIn` filter here, unlike the whitelist shown in
 * the Tags control). The two stay in sync from the moment the viewer makes
 * their first edit through `CasesFilterBar` onward: once they're
 * consciously picking specific tags to include, "what's displayed" and
 * "what's queried" collapse into the same value, and stay collapsed even
 * across a later Reset back to this baseline (see `handleReset`).
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
    if (isCatalogFetching) return null;
    if (isCatalogError || !tagCatalog) {
      // The catalog failed to load -- fall back to the widget's own raw
      // `excludeTags` rather than dropping it, so the search itself stays
      // correctly scoped (just excluded, not narrowed to a wrong "in"
      // list either) even though the "Tags" control has no way to *show*
      // an exclusion as checked items.
      return { ...DEFAULT_CASES_FILTERS, ...rawTranslated };
    }
    const excluded = new Set(rawTranslated.excludeTags);
    const complementTags = tagCatalog
      .map((t) => t.label)
      .filter((label) => !excluded.has(label));
    // A widget that also requires specific tags (`tag in [...]` alongside
    // `tag notIn [...]`) must keep both conditions ANDed -- intersect with
    // the complement rather than overwriting the required list outright,
    // or the widget's own "must have one of these tags" requirement is
    // silently lost.
    const priorTags = rawTranslated.tags;
    const tags =
      priorTags && priorTags.length > 0
        ? priorTags.filter((t) => complementTags.includes(t))
        : complementTags;
    return {
      ...DEFAULT_CASES_FILTERS,
      ...rawTranslated,
      tags,
      excludeTags: [],
    };
  }, [needsTagComplement, isCatalogFetching, isCatalogError, tagCatalog, rawTranslated]);

  // `initialCasesFilters` is reactive, not actually "initial" -- it goes
  // back to `null` whenever `isCatalogFetching` is true, including on a
  // *later* background refetch of `useSearchTags("", ...)`'s cache entry
  // (shared, by query key, with every other open "Tags" dropdown in the
  // app, e.g. the real one inside `CasesFilterBar` below, once its own
  // `staleTime` elapses). Freezing the first resolved value here, once,
  // is what makes both the initial seed below and `onReset` immune to that
  // later refetch -- without it, clicking Reset during that window would
  // fall back to `DEFAULT_CASES_FILTERS` and silently drop every one of
  // the widget's own starting filters, not just the tag complement.
  const [resetBaseline, setResetBaseline] = useState<CasesFilters | null>(null);
  if (resetBaseline === null && initialCasesFilters !== null) {
    setResetBaseline(initialCasesFilters);
  }

  const [casesFilters, setCasesFilters] = useState<CasesFilters | null>(null);
  // Seeds `casesFilters` from `resetBaseline` exactly once, as soon as it
  // resolves (immediately for a non-tag-complement widget; after the
  // catalog fetch settles otherwise) -- never re-seeds afterward, so it
  // can't clobber an edit the viewer already made while the catalog was
  // still loading.
  if (casesFilters === null && resetBaseline !== null) {
    setCasesFilters(resetBaseline);
  }

  // What actually gets queried -- the widget's real `excludeTags` blacklist,
  // never the catalog-derived complement `casesFilters` shows as checked
  // "Tags". Seeded eagerly from `rawTranslated` alone, so it's correct from
  // the very first render regardless of whether/when the tag catalog
  // resolves. Stays this way until the viewer edits the filter bar
  // themselves (`handleFiltersChange`), at which point the displayed value
  // becomes the source of truth for both.
  const [apiFilters, setApiFilters] = useState<CasesFilters>(() => ({
    ...DEFAULT_CASES_FILTERS,
    ...rawTranslated,
  }));

  const [isFiltersOpen, setIsFiltersOpen] = useState(true);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const handleFiltersChange = (next: CasesFilters): void => {
    setCasesFilters(next);
    setApiFilters(next);
    setPage(0);
  };

  // Distinct from `handleFiltersChange`: a Reset must restore the *original*
  // divergence (display shows the complement baseline, the query goes back
  // to the widget's real blacklist) rather than collapsing them the way a
  // manual edit does -- otherwise a single Reset after any edit would lock
  // `apiFilters` to the complement approximation forever.
  const handleReset = (): void => {
    setCasesFilters(resetBaseline ?? DEFAULT_CASES_FILTERS);
    setApiFilters({ ...DEFAULT_CASES_FILTERS, ...rawTranslated });
    setPage(0);
  };

  const handleRowsPerPageChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  const { data, isLoading, isError, isFetching, refetch, dataUpdatedAt } = useGetCsmCases(
    apiFilters,
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
          Couldn&rsquo;t load the tag catalog — this widget&rsquo;s tag exclusion is still
          applied to the results below, it just isn&rsquo;t shown as checked items in the Tags
          control.
        </Typography>
      )}
      <CasesFilterBar
        filters={casesFilters}
        onChange={handleFiltersChange}
        onReset={handleReset}
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

interface CaseFeedbackWidgetPreviewProps {
  widgetId: string;
  displayName: string;
  filters: Record<string, unknown>;
  backButton: JSX.Element;
}

/** The 5 case-feedback rating values, in order — same scale ServiceNow's own
 * survey uses (see `useCaseFeedbackTrendData`'s `colorForAvgRating` doc
 * comment for the same 1-5 -> CSAT-label mapping). No shared constant for
 * this exists elsewhere in the app (every other rating display reads the
 * label straight off the record itself), so it's scoped here rather than
 * invented as a new cross-feature export for a single dropdown. */
const FEEDBACK_RATING_OPTIONS: { value: string; label: string }[] = [
  { value: "1", label: "1 — Very Dissatisfied" },
  { value: "2", label: "2 — Dissatisfied" },
  { value: "3", label: "3 — Neutral" },
  { value: "4", label: "4 — Satisfied" },
  { value: "5", label: "5 — Very Satisfied" },
];

/** First string value out of a filter field that's either a bare string (a
 * tile/slice click-through's own scalar filters) or a 1-element string[]
 * (the same field once round-tripped through the preview URL — see
 * `parseWidgetPreviewFilters`, which decodes every param as a comma-split
 * array). Either shape lands here since `filters` (this component's own
 * prop) always came from that URL round trip. */
function asFeedbackFilterValue(v: unknown): string | undefined {
  if (typeof v === "string") return v;
  if (Array.isArray(v) && typeof v[0] === "string") return v[0];
  return undefined;
}

/**
 * The case-feedback "View more" landing: unlike the generic
 * `DashboardWidgetPreviewContent` fallback below (a static "Filtered by:"
 * chip summary plus a free-text search box the feedback search endpoint
 * doesn't even support — `case_feedback` has no `searchQuery` field), this
 * gives a real, editable rating + date-range filter bar, seeded from the
 * widget's own filters (a rating-pie or trend-bar slice click-through) and
 * then freely adjustable from there — the same "seeded then editable"
 * pattern `CaseFamilyWidgetPreview` already uses for the case-family
 * resourceTypes, scoped to this resourceType's own flat
 * `dateFrom`/`dateTo`/`rating` filter shape (see
 * `WIDGET_RESOURCE_CONFIG.case_feedback`'s own `buildSearchRequestBody`
 * doc comment) rather than the case-search DSL that component translates.
 */
function CaseFeedbackWidgetPreview({
  widgetId,
  displayName,
  filters,
  backButton,
}: CaseFeedbackWidgetPreviewProps): JSX.Element {
  // Frozen once at mount (a fresh "View more"/slice click is a fresh mount),
  // same rationale as `CaseFamilyWidgetPreview`'s `resetBaseline` — Reset
  // must restore what the widget/slice actually linked here with, not
  // whatever the viewer has since edited.
  const [initial] = useState(() => ({
    dateFrom: asFeedbackFilterValue(filters.dateFrom),
    dateTo: asFeedbackFilterValue(filters.dateTo),
    rating: asFeedbackFilterValue(filters.rating),
  }));
  const [dateRange, setDateRange] = useState<DateRangeFilterValue>({
    from: initial.dateFrom,
    to: initial.dateTo,
  });
  const [rating, setRating] = useState<string>(initial.rating ?? "");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const queriedFilters = useMemo(() => {
    const out: Record<string, unknown> = { ...filters };
    delete out.dateFrom;
    delete out.dateTo;
    delete out.rating;
    if (dateRange.from) out.dateFrom = dateRange.from;
    if (dateRange.to) out.dateTo = dateRange.to;
    if (rating) out.rating = Number(rating);
    return out;
  }, [filters, dateRange, rating]);

  const handleReset = (): void => {
    setDateRange({ from: initial.dateFrom, to: initial.dateTo });
    setRating(initial.rating ?? "");
    setPage(0);
  };

  const { data, isLoading, isError, isFetching, refetch, dataUpdatedAt } = useWidgetData(
    widgetId,
    "case_feedback",
    queriedFilters,
    "list",
    rowsPerPage,
    page * rowsPerPage,
  );
  const ListRenderer = WIDGET_LIST_RENDERERS.case_feedback;

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
      <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "flex-end", gap: 2 }}>
        <DateRangeFilter
          label="Feedback submitted"
          value={dateRange}
          onChange={(next) => {
            setDateRange(next);
            setPage(0);
          }}
        />
        <FormControl size="small" sx={{ minWidth: 220 }}>
          <InputLabel id="feedback-rating-filter-label">Rating</InputLabel>
          <Select
            labelId="feedback-rating-filter-label"
            label="Rating"
            value={rating}
            onChange={(e) => {
              setRating(e.target.value);
              setPage(0);
            }}
          >
            <MenuItem value="">All ratings</MenuItem>
            {FEEDBACK_RATING_OPTIONS.map((option) => (
              <MenuItem key={option.value} value={option.value}>
                {option.label}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <Button variant="text" size="small" onClick={handleReset}>
          Reset
        </Button>
      </Box>
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
