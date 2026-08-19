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
 * What kind of lookup a typed case-search string should run as:
 * - `"number"` / `"internalId"`: an exact-match, first-class filter — the
 *   entity-service resolves these against an indexed column instead of the
 *   free-text `searchQuery` scan. Named to match the response field names
 *   (`BeCaseSearchView.number` / `.internalId`), not a UI-facing label.
 * - `"text"`: the existing free-text search across subject/description
 *   (and number/internalId, case-insensitively) — unchanged behavior.
 */
export type CaseQueryScope = "number" | "internalId" | "text";

/**
 * Classifies a typed (already-trimmed) case-search string into the scope it
 * should be routed through. Order matters: a case number is checked first
 * since `CS\d{7}` is a strict subset of the looser WSO2-case-id shape.
 *
 * Shared deliberately: the global quick-nav palette
 * (`useQuickCaseSearch`) and the cases-list / Support-page search
 * (`buildCaseSearchFilters`) must classify identically, or the same typed
 * string resolves one case in the palette and a different one in the list —
 * which is exactly the bug this consolidation fixes.
 */
export function classifyCaseQuery(query: string): CaseQueryScope {
  if (CASE_NUMBER_RE.test(query)) return "number";
  if (WSO2_CASE_ID_RE.test(query)) return "internalId";
  return "text";
}
