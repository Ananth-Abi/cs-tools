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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import { severityFromBe, uiStateFromBe } from "@api/backend/mappers";
import type {
  BeCaseFieldFilter,
  BeCaseSearchFilters,
  BeCaseSearchPayload,
  BeCaseSearchResponse,
  BeCaseType,
} from "@api/backend/types";
import type {
  CaseState,
  CaseWorkState,
  SeverityOrUnset,
} from "@features/csm-dashboard/types/abtDashboard";
import { ALL_CASE_TYPES } from "@features/csm-cases/utils/caseType";

/** Don't fire a search until the user has typed something searchable. */
export const QUICK_CASE_MIN_QUERY_LEN = 2;

/** A small result page — the palette only shows the top few hits. */
const QUICK_CASE_LIMIT = 8;

/** A CS case number, e.g. "CS0441174" — always "CS" plus exactly 7 digits. */
const CASE_NUMBER_RE = /^CS\d{7}$/;

/**
 * A WSO2 case id, e.g. "SOMEID-4" — an alphanumeric project/product prefix, a
 * hyphen, then 1-4 digits. Deliberately looser than {@link CASE_NUMBER_RE}
 * (no fixed prefix), so it's checked second, after the case-number pattern
 * has already had first refusal.
 */
const WSO2_CASE_ID_RE = /^[a-zA-Z0-9]+-\d{1,4}$/;

/**
 * What kind of lookup a typed quick-search string should run as:
 * - `"number"` / `"internalId"`: an exact-match, first-class filter — the
 *   entity-service resolves these against an indexed column instead of the
 *   free-text `searchQuery` scan. Named to match the response field names
 *   (`BeCaseSearchView.number` / `.internalId`), not a UI-facing label.
 * - `"text"`: the existing free-text search across subject/description
 *   (and number/internalId, case-insensitively) — unchanged behavior.
 */
export type QuickCaseSearchScope = "number" | "internalId" | "text";

/**
 * Classifies a typed (already-trimmed) quick-search string into the scope
 * {@link useQuickCaseSearch} should route it through. Order matters: a case
 * number is checked first since `CS\d{7}` is a strict subset of the looser
 * WSO2-case-id shape.
 */
export function classifyQuickCaseQuery(query: string): QuickCaseSearchScope {
  if (CASE_NUMBER_RE.test(query)) return "number";
  if (WSO2_CASE_ID_RE.test(query)) return "internalId";
  return "text";
}

/**
 * One hit from the global-search case lookup. Carries the UUID `id` (for the
 * `/cases/:id` link), the human-readable identity/subject, and enough of the
 * case's severity/status/ownership for the palette to render a result card
 * matching the case list's visual language.
 */
export interface QuickCaseHit {
  id: string;
  caseNumber?: string;
  wso2CaseId?: string;
  subject: string;
  severity: SeverityOrUnset;
  state: CaseState;
  workState?: CaseWorkState | null;
  caseType?: BeCaseType;
  updatedOn?: string;
  createdOn?: string;
  assigneeName?: string;
}

/**
 * Case lookup for the global quick-nav palette. Calls `POST /cases/search`,
 * routing the typed text one of three ways (see {@link classifyQuickCaseQuery}):
 * a `CS\d{7}` case number or a `<prefix>-<digits>` WSO2 case id goes through
 * as an exact-match, first-class field filter (an indexed lookup); anything
 * else falls back to the same free-text `searchQuery` search the cases list
 * uses. Pass `forceFreeText` to opt back into the free-text path even when
 * the query matches one of the exact patterns — the quick-nav palette's
 * "search in subject and description too" affordance uses this to widen a scoped result.
 *
 * Explicitly requests every known case sub-type ({@link ALL_CASE_TYPES}) —
 * `entity-service`'s `/cases/search` defaults `filters.types` to `["case"]`
 * only when the caller omits it, which silently hid Service Requests,
 * Security Report Analyses, announcements, and engagements from this search.
 *
 * The query is disabled until the trimmed text reaches
 * {@link QUICK_CASE_MIN_QUERY_LEN}, so opening the palette costs no network.
 */
export function useQuickCaseSearch(
  query: string,
  options?: { forceFreeText?: boolean },
): UseQueryResult<QuickCaseHit[], Error> {
  const api = useBackendApi();
  const q = query.trim();
  const scope = options?.forceFreeText ? "text" : classifyQuickCaseQuery(q);

  return useQuery<QuickCaseHit[], Error>({
    queryKey: [ApiQueryKeys.CSM_CASES, "quick-search", q, scope],
    queryFn: async (): Promise<QuickCaseHit[]> => {
      const typeFilter: BeCaseFieldFilter = {
        field: "type",
        op: "in",
        values: ALL_CASE_TYPES,
      };
      const filters: BeCaseSearchFilters =
        scope === "text"
          ? { searchQuery: q, filters: [typeFilter] }
          : {
              filters: [
                typeFilter,
                { field: scope, op: "eq", values: [q] },
              ],
            };
      const res = await api.post<BeCaseSearchPayload, BeCaseSearchResponse>(
        "/cases/search",
        {
          pagination: { offset: 0, limit: QUICK_CASE_LIMIT },
          filters,
        },
      );
      return (res.cases ?? []).map((c) => ({
        id: c.id,
        caseNumber: c.number,
        wso2CaseId: c.internalId,
        subject: c.subject ?? "(no subject)",
        severity: severityFromBe(c.severity),
        state: uiStateFromBe(c.state),
        workState: c.workState,
        caseType: c.type,
        updatedOn: c.updatedOn,
        createdOn: c.createdOn,
        assigneeName: c.assignedEngineer?.name,
      }));
    },
    enabled: q.length >= QUICK_CASE_MIN_QUERY_LEN,
    staleTime: 15_000,
  });
}
