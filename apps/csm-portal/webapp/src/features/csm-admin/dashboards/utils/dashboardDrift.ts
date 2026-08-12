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

import type { BeDashboard, BeDashboardWidget } from "@api/backend/types";
import type { DashboardDraft } from "@features/csm-admin/dashboards/utils/dashboardDraftsStorage";

/**
 * Deterministic stringification (object keys sorted recursively) so two
 * structurally-equal-but-differently-ordered objects compare equal — a
 * plain `JSON.stringify` would treat `{a:1,b:2}` and `{b:2,a:1}` as
 * different, which would false-positive a drift warning on every draft
 * whose widgets/filters happen to have been rebuilt in a different key
 * order than what the backend returns.
 */
function canonicalize(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(canonicalize);
  if (value && typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const key of Object.keys(value as Record<string, unknown>).sort()) {
      out[key] = canonicalize((value as Record<string, unknown>)[key]);
    }
    return out;
  }
  return value;
}

function canonicalJson(value: unknown): string {
  return JSON.stringify(canonicalize(value));
}

/** The subset of `BeDashboard`/`DashboardDraft` that actually gets deployed
 * — comparing on this (rather than the full draft object, which also
 * carries builder-only bookkeeping like `id`/`sourceDashboardId`/
 * `emptySections`/`updatedAt`) is what makes the comparison meaningful:
 * those fields differ by construction and would make every draft look
 * perpetually drifted. */
interface DeployableDashboardShape {
  displayName: string;
  isDefault: boolean;
  isTeamBased: boolean;
  targetTeam?: string;
  widgets: BeDashboardWidget[];
}

// `type` is deliberately excluded: it's on `BeDashboardListItem` (the list
// response) but NOT on `BeDashboard` (the detail response this compares
// against) — comparing it would false-positive drift on every draft, since
// there's nothing on the live side to compare it to.
function deployableShapeFromDraft(draft: DashboardDraft): DeployableDashboardShape {
  return {
    displayName: draft.displayName,
    isDefault: draft.isDefault,
    isTeamBased: draft.isTeamBased,
    targetTeam: draft.targetTeam,
    widgets: draft.widgets,
  };
}

function deployableShapeFromLive(live: BeDashboard): DeployableDashboardShape {
  return {
    displayName: live.displayName,
    isDefault: live.isDefault,
    isTeamBased: live.isTeamBased,
    targetTeam: live.targetTeam,
    widgets: live.widgets,
  };
}

/**
 * True when `draft` no longer matches what `GET /dashboards/{id}` currently
 * returns — i.e. the local draft has unsaved-to-deployment changes. Always
 * `true` for a draft with no `sourceDashboardId` (nothing deployed to
 * compare against): a brand-new dashboard is drifted from "deployed" by
 * definition until a maintainer first ships it.
 */
export function isDraftDrifted(draft: DashboardDraft, live: BeDashboard | undefined): boolean {
  if (!live) return true;
  // A draft not yet tied to ANY deployed dashboard is drifted by
  // definition (see this function's own doc comment above) — checked
  // before the shape comparison below, so a caller that happens to pass a
  // live dashboard alongside a draft that was never actually opened FROM
  // it (e.g. matched only by a shared id) can't fall through to a
  // content-equality check that was never the right comparison to begin
  // with.
  if (!draft.sourceDashboardId) return true;
  return canonicalJson(deployableShapeFromDraft(draft)) !== canonicalJson(deployableShapeFromLive(live));
}
