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

import { isCaseFieldFilterArray, type WidgetCaseFieldFilterLike } from "./widgetPreviewUrl";

/**
 * Placeholder value a `creTeam` or `sreTeam` filter entry's `values` array
 * may carry — mirrors how `assignedUserId` widgets carry the signed-in user's
 * own placeholder and how the entity-service resolves its own
 * `__current_user_email__` for `createdBy` server-side. Unlike both of
 * those, this one is resolved entirely CLIENT-SIDE: it stands for "the
 * currently selected team in this dashboard's own UI state", not anything
 * about the signed-in user's identity, so only this frontend (never the
 * entity-service) ever sees it.
 */
export const CURRENT_TEAM_PLACEHOLDER = "__current_team__";

/**
 * Sentinel `selectedTeamId`/`selectedTeamCreGroupId`/`selectedTeamSreGroupId`
 * value for the team picker's "All ABTs" option (see `AbtDashboardHeader`) —
 * every team in the current dashboard's own family
 * (`abtFamilyForDashboardType`), OR'd together, rather than exactly one
 * team's group id. `__`-prefixed so it can never collide with a real
 * `BeTeam.id` (registry team keys like
 * `"castor"` never use that convention). This is deliberately narrower than
 * the "My ABT / All customers" toggle removed 2026-08-02 (see
 * `AbtDashboardHeader`'s own doc comment) — that toggle spanned every team
 * in the whole registry; this one never leaves the current dashboard's
 * family.
 */
export const ALL_TEAMS_SENTINEL = "__all__";

export const CRE_TEAM_FILTER_FIELD = "creTeam";
export const SRE_TEAM_FILTER_FIELD = "sreTeam";

/**
 * Substitutes {@link CURRENT_TEAM_PLACEHOLDER} wherever it appears in a
 * `creTeam` or `sreTeam` filter entry's `values` (in the same
 * `{ filters: BeCaseFieldFilter[] }` DSL shape `mergeWidgetFilters` and
 * `resolveCurrentUserSentinels` already walk) with the selected team's own
 * group id for that discipline — the backing data source's assignment-group
 * id reformatted as this platform's UUID (`BeTeam.creGroupId` for a
 * `creTeam` entry, `BeTeam.sreGroupId` for an `sreTeam` entry), never the
 * team registry key (`BeTeam.id`) that neither field's values are keyed by.
 * The two fields are resolved entirely independently — a `creTeam` entry is
 * only ever substituted from `selectedTeamCreGroupId`, an `sreTeam` entry
 * only from `selectedTeamSreGroupId`, so a single dashboard could in theory
 * carry both filter kinds across different widgets and each would resolve
 * from its own discipline's group id. This only applies when the
 * corresponding group id argument is a single selected team (a plain
 * string).
 *
 * Either group id argument may also be an array of group ids — the "All
 * ABTs" case (see {@link ALL_TEAMS_SENTINEL}). As of 2026-08-05 this is an
 * explicit, deliberate product decision: the matching entry (`creTeam` for
 * `selectedTeamCreGroupId`, `sreTeam` for `selectedTeamSreGroupId`) is
 * DROPPED from the filter array entirely whenever that argument is an
 * array, regardless of its contents, rather than being replaced with the
 * enumerated list of every team's group id in the current dashboard's
 * family. Dropping the filter widens the query to *every* team in the whole
 * registry — including non-ABT teams (`cre`, `sre`, etc.) outside this
 * dashboard's own family — which is broader than "every team in this
 * family" and was previously avoided on purpose (see the removed "My ABT /
 * All customers" toggle this replaced). That tradeoff was considered and
 * accepted: "All ABTs" now means "no team filter at all", org-wide.
 *
 * If a group id argument is undefined, or an array (see above) — no team
 * selected yet, the selected team has no group configured in the
 * deployment's team registry for that discipline, or "All ABTs" is selected
 * — the corresponding filter entry is DROPPED from the filter array
 * entirely, rather than either (a) sent with the literal placeholder
 * string, which the entity-service would either reject with a 400 (not a
 * valid UUID) or, worse, silently treat as a value that matches nothing, or
 * (b) sent with an empty `values` array, which the entity-service also
 * rejects for a non-`isEmpty`/`isNotEmpty` op. Dropping the condition
 * instead just widens the query back to "every team" — the same result as
 * if this filter had never been applied — which is the safer failure mode
 * for a dashboard tile: a count/list that's too broad is visibly wrong (an
 * obviously large number, or rows from other teams) and gets noticed, where
 * a query that silently matches zero rows reads as "there's nothing to see
 * here" and doesn't.
 *
 * Every other filter entry, and every other resourceType's filters shape
 * (this only touches the case-search generic field/op/values DSL), passes
 * through unchanged.
 */
export function resolveTeamPlaceholder(
  filters: Record<string, unknown>,
  selectedTeamCreGroupId: string | string[] | undefined,
  selectedTeamSreGroupId: string | string[] | undefined,
): Record<string, unknown> {
  // Callers pass widget/slice `filters` straight from backend-driven config
  // (DASHBOARDS_CONFIG, a raw JSON env var not schema-validated beyond basic
  // decoding) — genuinely absent at runtime is possible despite the wire
  // type declaring it required.
  filters ??= {};
  const fieldFilters = filters.filters;
  if (!isCaseFieldFilterArray(fieldFilters)) return filters;

  const replacementCreGroupId =
    typeof selectedTeamCreGroupId === "string" ? selectedTeamCreGroupId : undefined;
  const replacementSreGroupId =
    typeof selectedTeamSreGroupId === "string" ? selectedTeamSreGroupId : undefined;

  let changed = false;
  const resolved: WidgetCaseFieldFilterLike[] = [];
  for (const entry of fieldFilters) {
    const values = entry.values;
    const isCreEntry = entry.field === CRE_TEAM_FILTER_FIELD;
    const isSreEntry = entry.field === SRE_TEAM_FILTER_FIELD;
    if ((!isCreEntry && !isSreEntry) || !values?.includes(CURRENT_TEAM_PLACEHOLDER)) {
      resolved.push(entry);
      continue;
    }
    changed = true;
    const replacementGroupId = isCreEntry ? replacementCreGroupId : replacementSreGroupId;
    if (replacementGroupId === undefined) {
      // Drop the entry entirely — see the doc comment above.
      continue;
    }
    resolved.push({
      ...entry,
      values: values.flatMap((v) => (v === CURRENT_TEAM_PLACEHOLDER ? [replacementGroupId] : [v])),
    });
  }

  if (!changed) return filters;
  return { ...filters, filters: resolved };
}
