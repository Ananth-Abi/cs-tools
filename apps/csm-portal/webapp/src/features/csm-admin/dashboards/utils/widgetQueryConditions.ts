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

import type { BeCaseFieldFilterOp, BeWidgetResourceType } from "@api/backend/types";

/**
 * A widget's `query` (opaque to every other part of this app — see
 * `BeDashboardWidget.query`) is NOT one uniform shape across every
 * resourceType's own `POST /{resourceType}s/search` contract:
 *
 * - `case` and its four `type`-variant resourceTypes (`service_request`,
 *   `security_report_analysis`, `announcement`, `engagement`) all route to
 *   `/cases/search`, whose filters are the generic field/op/values DSL
 *   nested under `query.filters` (see `BeCaseFieldFilter`).
 * - Every other resourceType (`incident`, `change_request`, `account`, …)
 *   has its own bespoke named-field filter shape, flat under `query`
 *   itself (e.g. `BeIncidentSearchPayload.filters.priorities`) — there is
 *   no single generic DSL for these anywhere in this app.
 *
 * This module gives the widget editor ONE condition-row UI (field,
 * operator, value(s)) that round-trips through whichever of those two
 * shapes actually matches the widget's own `resourceType`, rather than
 * forcing every resourceType's filters into the case DSL (which its real
 * search endpoint would reject) or exposing raw JSON.
 */

export type FilterConditionOp = BeCaseFieldFilterOp;

export const FILTER_CONDITION_OPS: FilterConditionOp[] = [
  "eq",
  "in",
  "notIn",
  "gte",
  "lte",
  "isEmpty",
  "isNotEmpty",
];

/** One editable filter row. `values` is ignored for `isEmpty`/`isNotEmpty`
 * (those two ops are value-less predicates — see `BeCaseFieldFilter`). */
export interface FilterCondition {
  field: string;
  op: FilterConditionOp;
  values: string[];
}

const NO_VALUE_OPS = new Set<FilterConditionOp>(["isEmpty", "isNotEmpty"]);

/** resourceTypes that route to `/cases/search` and therefore use the
 * generic case field/op/values DSL — see this module's own doc comment. */
const CASE_FIELD_DSL_RESOURCE_TYPES = new Set<BeWidgetResourceType>([
  "case",
  "service_request",
  "security_report_analysis",
  "announcement",
  "engagement",
]);

export function usesCaseFieldFilterDsl(resourceType: BeWidgetResourceType): boolean {
  return CASE_FIELD_DSL_RESOURCE_TYPES.has(resourceType);
}

/** Every field the case-search DSL accepts (mirrors `BeCaseFieldFilterField`
 * — see `types.ts`), offered as autocomplete suggestions in the field
 * picker for a case-like resourceType. Freeform text is still accepted:
 * this is a suggestion list, not a hard allowlist, since the backend (not
 * this list) is the source of truth for what it accepts. */
export const CASE_FIELD_OPTIONS: string[] = [
  "type",
  "state",
  "severity",
  "engagementType",
  "issueType",
  "workState",
  "tag",
  "projectId",
  "deploymentId",
  "assignedUserId",
  "createdBy",
  "createdOn",
  "updatedOn",
  "closedOn",
  "product",
  "projectOnboardingStatus",
  "projectType",
  "integrationCsTeam",
  "resolutionNotes",
  "parentId",
  "taskSLABusinessElapsedPercent",
  "escalationLevel",
  "escalation",
  "number",
  "internalId",
];

function isFilterOp(v: unknown): v is FilterConditionOp {
  return typeof v === "string" && (FILTER_CONDITION_OPS as string[]).includes(v);
}

/** Reads a widget's own `query` into editable condition rows, per
 * `usesCaseFieldFilterDsl`. An unrecognized/malformed entry is skipped
 * rather than crashing the editor — the admin can always still delete/
 * retype a row that came out empty. */
export function filterConditionsFromQuery(
  resourceType: BeWidgetResourceType,
  query: Record<string, unknown> | undefined,
): FilterCondition[] {
  if (!query) return [];

  if (usesCaseFieldFilterDsl(resourceType)) {
    const raw = query.filters;
    if (!Array.isArray(raw)) return [];
    return raw
      .filter((e): e is Record<string, unknown> => !!e && typeof e === "object")
      .map((e) => ({
        field: typeof e.field === "string" ? e.field : "",
        op: isFilterOp(e.op) ? e.op : "eq",
        values: Array.isArray(e.values) ? e.values.map(String) : [],
      }))
      .filter((c) => c.field.length > 0);
  }

  // Every other resourceType's own search contract is flat named top-level
  // keys, not this app's field/op/values DSL — one row per key. `in` for an
  // array value (e.g. `priorities: ["HIGH"]`), `eq` for a scalar (e.g.
  // `number: "INC0090472"`).
  return Object.entries(query)
    .filter(([, v]) => v !== undefined && v !== null)
    .map(([field, v]) => ({
      field,
      op: (Array.isArray(v) ? "in" : "eq") as FilterConditionOp,
      values: Array.isArray(v) ? v.map(String) : [String(v)],
    }));
}

/** The inverse of `filterConditionsFromQuery` — serializes edited condition
 * rows back into the `query` shape that resourceType's own search endpoint
 * actually accepts. Rows with an empty `field` are dropped. */
export function queryFromFilterConditions(
  resourceType: BeWidgetResourceType,
  conditions: FilterCondition[],
): Record<string, unknown> {
  const valid = conditions.filter((c) => c.field.trim().length > 0);

  if (usesCaseFieldFilterDsl(resourceType)) {
    if (valid.length === 0) return {};
    return {
      filters: valid.map((c) =>
        NO_VALUE_OPS.has(c.op)
          ? { field: c.field, op: c.op }
          : { field: c.field, op: c.op, values: c.values },
      ),
    };
  }

  const out: Record<string, unknown> = {};
  for (const c of valid) {
    // Non-case resourceTypes' own contracts only ever use a scalar or an
    // array (see this module's own doc comment) — `in` writes the array,
    // every other op collapses to a single scalar value (the first one
    // entered), since none of those endpoints understand
    // isEmpty/isNotEmpty/gte/lte as a bare top-level key.
    out[c.field] = c.op === "in" ? c.values : (c.values[0] ?? "");
  }
  return out;
}
