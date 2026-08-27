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

import { useQuery } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeCaseFeedbackAggregateResponse, BeDashboardGroupByConfig } from "@api/backend/types";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import { resolveRelativeDateFilters } from "@features/csm-dashboard/utils/resolveRelativeDateFilters";
import {
  shouldRetryWidgetFetch,
  withWidgetFetchSlot,
} from "@features/csm-dashboard/utils/widgetFetchConcurrency";
import type { PieSliceResult, WidgetPieData } from "@features/csm-dashboard/api/useWidgetPieData";

/** Formats one bucket's own `bucketStart` for the x-axis label, at the
 * granularity implied by `bucket` — "month" reads as "Aug 2026" (the day is
 * meaningless at that granularity), "week"/"day" as "Aug 1" (the year is
 * omitted; a trend chart spans at most a handful of months in practice, and
 * dropping it keeps the label short enough not to overlap its neighbors).
 * Falls back to the raw string for anything `Date` can't parse, rather than
 * rendering "Invalid Date" — the entity service is the source of this value,
 * not something validated client-side.
 *
 * Formatted in `timeZone: "UTC"`, deliberately NOT the viewer's own local
 * time (unlike `resolveRelativeDateFilters`, which is intentionally
 * local-time for a different reason — see that module's own doc comment): a
 * date-only `bucketStart` like `"2026-08-01"` parses as UTC midnight, and
 * reading it back in a negative-UTC-offset timezone (most of the Americas)
 * without pinning the formatter to UTC would roll it back to the previous
 * day/month on the label — e.g. "Jul 2026" for a bucket the response itself
 * calls August. Pinning to UTC makes the label match the bucket's own wire
 * value everywhere, regardless of the viewer's timezone. */
function formatBucketLabel(bucketStart: string, bucket: "day" | "week" | "month"): string {
  const date = new Date(bucketStart);
  if (Number.isNaN(date.getTime())) return bucketStart;
  return new Intl.DateTimeFormat("en-US", {
    timeZone: "UTC",
    month: "short",
    year: bucket === "month" ? "numeric" : undefined,
    day: bucket === "month" ? undefined : "numeric",
  }).format(date);
}

/**
 * Resolves a `shape: "bar"` `case_feedback` widget's date-bucketed rating
 * trend via a single `POST /cases/feedback/aggregate` call — the
 * `groupBy.bucket` counterpart of `useWidgetGroupByData`'s `groupBy.field`
 * (see that field's own doc comment on `BeDashboardGroupByConfig` for why
 * this is a dedicated hook rather than a branch inside that one: the
 * request body (`{filters, bucket}`, no `groupBy`/`maxGroups`) and response
 * shape (`{buckets: [{bucketStart, avgRating, count}], totalRecords}`, no
 * `groups`/`othersCount`) are both unrelated to `BeGroupByResponse`).
 *
 * Renders **average rating per bucket** as the primary metric (this task's
 * own stated default) rather than a count-by-rating-bucket breakdown — every
 * returned slice's `value` is `avgRating` (1-5), not `count`. `count` is
 * still read off the response (as `WidgetPieData.total`, the bar tile's own
 * header badge — see `DashboardWidgetTile`), just not per-bucket.
 *
 * Every bucket slice is marked `navigable: false`: unlike a field-based
 * groupBy bucket (which resolves to a real filtered case-list query — see
 * `useWidgetGroupByData`'s `bucketQuery`), a case-feedback bucket has no
 * dedicated list page or click-through target of its own to scope a filtered
 * result set to (see `WIDGET_RESOURCE_CONFIG.case_feedback.buildHref`) — a
 * navigable-but-nowhere-useful click was judged worse than none.
 *
 * `groupBy` of `undefined`, or one carrying no `bucket` (a field-based
 * `groupBy` reaching this hook by mistake — never true in practice, since
 * `DashboardWidgetTile` only calls this hook when `groupBy.bucket` is set),
 * fires no query and returns an empty/zero result, mirroring
 * `useWidgetGroupByData`'s own behavior for an undefined `groupBy`.
 */
export function useCaseFeedbackTrendData(
  widgetId: string,
  /** This widget's own base filters (`CaseFeedbackAggregateFilters` —
   * `accountIds`/`dateFrom`/`dateTo`; NOT the case-search DSL). Already
   * resolved for the `__dateRangeFrom__`/`__dateRangeTo__` placeholder by
   * the caller (`DashboardWidgetGrid`'s `renderTile` — see
   * `dateRangeFilterPlaceholder.ts`), the single merge point shared with the
   * `shape: "list"` grid widget that reads its own filters off the exact
   * same `widget.query`, so this hook only ever sees concrete values (or
   * none). Only `resolveRelativeDateFilters` runs here — a fail-open no-op
   * for this resourceType's own flat `dateFrom`/`dateTo` shape (it only
   * resolves the case-search DSL's `{filters: [...]}`  shape), kept for
   * parity with every other widget hook and as a forward-compatible no-op
   * should a relative-date placeholder ever be supported here too. */
  baseFilters: Record<string, unknown>,
  groupBy: BeDashboardGroupByConfig | undefined,
  enabled = true,
): WidgetPieData {
  const api = useBackendApi();
  const config = WIDGET_RESOURCE_CONFIG.case_feedback;
  const bucket = groupBy?.bucket;

  const resolvedFilters = resolveRelativeDateFilters(baseFilters);

  const query = useQuery({
    queryKey: [
      ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA,
      "feedback-trend",
      widgetId,
      resolvedFilters,
      bucket,
    ],
    queryFn: async (): Promise<BeCaseFeedbackAggregateResponse> => {
      if (!bucket) {
        return { buckets: [], totalRecords: 0 };
      }
      if (!config?.groupByEndpoint) {
        throw new Error("case_feedback resourceType has no groupByEndpoint configured");
      }
      // Same shared concurrency slot (and timeout) every other widget fetch
      // uses — see useWidgetGroupByData's own comment. This resourceType has
      // no team concept at all (see the retry option's own comment below), so
      // a constant `teamKey` ("case_feedback") is passed — it never drops
      // this fetch from the shared FIFO queue.
      return withWidgetFetchSlot(async (signal) => {
        return api.post<
          { filters: Record<string, unknown>; bucket: "day" | "week" | "month" },
          BeCaseFeedbackAggregateResponse
        >(
          config.groupByEndpoint as string,
          { filters: resolvedFilters, bucket },
          { signal },
        );
      }, "case_feedback");
    },
    enabled: enabled && !!bucket,
    // This dashboard carries no team/current-user placeholder at all (see
    // `dateRangeFilterPlaceholder.ts` — case_feedback's own filters shape has
    // no team-scoped field), so every fetch here is "team-independent" in
    // `shouldRetryWidgetFetch`'s own sense — always eligible for the
    // queue-drop retry it applies to a widget whose own filters can't change
    // out from under an in-flight fetch.
    retry: (failureCount, error) => shouldRetryWidgetFetch(failureCount, error, true),
    staleTime: 60_000,
  });

  const isLoading = !enabled || (!!bucket && query.isLoading);
  const isError = !!bucket && query.isError;

  const buckets = query.data?.buckets ?? [];
  const slices: PieSliceResult[] = buckets.map((b) => ({
    label: bucket ? formatBucketLabel(b.bucketStart, bucket) : b.bucketStart,
    query: {},
    navigable: false,
    value: b.avgRating,
  }));
  const total = query.data?.totalRecords ?? 0;

  return { slices, total, isLoading, isError };
}
