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

/** The five case-like record types that share `CsmCaseDetailPage`. */
export type CaseRouteKind =
  | "case"
  | "service_request"
  | "engagement"
  | "announcement"
  | "security_report_analysis";

export interface CaseTabState {
  /** Stable synthetic id for this open tab, assigned once at open time.
   * Deliberately NOT the same value as `caseId`: an in-tab navigation to a
   * different case (see `CaseTabIsolatedRouter`) always opens/activates a
   * *different* tab rather than mutating this one in place, so `id` never
   * needs to change for the lifetime of a tab — which keeps the tab a stable
   * React list key and avoids remounting `CsmCaseDetailPage` on every path
   * change inside it (the whole point of keeping tabs alive). */
  id: string;
  caseId: string;
  kind: CaseRouteKind;
  /** Current concrete path for this tab, e.g. "/cases/CS0001". Updated in
   * place (not via a tab-identity change) by in-tab navigation that resolves
   * to the SAME caseId — the misrouted-case redirect inside
   * `CsmCaseDetailPage`, or the dashless-id repair in `useNormalizedIdParam`. */
  path: string;
  /** Short display label ("CS0001 · Subject line"); undefined until the
   * case's own data has loaded once. */
  label?: string;
  /** Best-effort signal that this case's reply composer is open, used only
   * to decide whether closing this tab needs a confirm — see
   * `CaseTabsContext`'s `reportDraftState`. */
  hasDraft: boolean;
  /** Router `location.state` at the moment this tab was opened — carries the
   * originating list's filtered URL (`{ from: string }`) so the page's own
   * Back button returns to it. Captured once at open time only (not updated
   * by later in-tab navigation); not persisted to sessionStorage, so a
   * reload falls back to the page's own hardcoded backPath instead — see
   * `CaseTabsContext`'s persistence code. */
  state?: unknown;
}

export interface CaseTabsPersistedState {
  tabs: { id: string; caseId: string; kind: CaseRouteKind; path: string }[];
  activeTabId: string | null;
}

/** Hard cap on simultaneously open in-app case tabs (browser-tab-strip
 * parity target; see the feature's own design notes for why 5 and why
 * "block" rather than evict-LRU). */
export const MAX_OPEN_CASE_TABS = 5;
