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

import { describe, expect, it } from "vitest";
import { isDraftDrifted } from "@features/csm-admin/dashboards/utils/dashboardDrift";
import type { DashboardDraft } from "@features/csm-admin/dashboards/utils/dashboardDraftsStorage";
import type { BeDashboard } from "@api/backend/types";

const WIDGET = {
  widgetId: "w1",
  displayName: "My Widget",
  resourceType: "case",
  shape: "count",
  gridWidth: 3,
  query: { filters: [{ field: "state", op: "in", values: ["open"] }] },
} as const;

function draft(overrides: Partial<DashboardDraft> = {}): DashboardDraft {
  return {
    id: "d1",
    sourceDashboardId: "live1",
    displayName: "Engineer overview",
    isDefault: false,
    isTeamBased: false,
    widgets: [WIDGET],
    emptySections: [],
    updatedAt: "2026-08-11T00:00:00.000Z",
    ...overrides,
  };
}

function live(overrides: Partial<BeDashboard> = {}): BeDashboard {
  return {
    id: "live1",
    displayName: "Engineer overview",
    isDefault: false,
    isTeamBased: false,
    widgets: [WIDGET],
    ...overrides,
  };
}

describe("isDraftDrifted", () => {
  it("is not drifted when the draft matches the live dashboard exactly", () => {
    expect(isDraftDrifted(draft(), live())).toBe(false);
  });

  it("is not drifted merely because keys are in a different order", () => {
    const reordered: BeDashboard = {
      widgets: [WIDGET],
      isTeamBased: false,
      isDefault: false,
      displayName: "Engineer overview",
      id: "live1",
    };
    expect(isDraftDrifted(draft(), reordered)).toBe(false);
  });

  it("is drifted when a widget's filters differ", () => {
    const changed = draft({
      widgets: [{ ...WIDGET, query: { filters: [{ field: "state", op: "in", values: ["closed"] }] } }],
    });
    expect(isDraftDrifted(changed, live())).toBe(true);
  });

  it("is drifted when displayName differs", () => {
    expect(isDraftDrifted(draft({ displayName: "Renamed" }), live())).toBe(true);
  });

  it("is always drifted (nothing deployed to compare against) when there's no live dashboard", () => {
    expect(isDraftDrifted(draft({ sourceDashboardId: undefined }), undefined)).toBe(true);
  });

  it("is drifted when there's no sourceDashboardId, even if a live dashboard is passed and matches by content", () => {
    // A draft that was never actually opened FROM a deployed dashboard is
    // "not yet tied to any deployed dashboard" (this function's own doc
    // comment) — a caller passing a live dashboard alongside it anyway
    // (e.g. matched only by a shared id) must not fall through to a
    // content-equality check.
    const neverDeployed = draft({ sourceDashboardId: undefined });
    expect(isDraftDrifted(neverDeployed, live())).toBe(true);
  });

  it("ignores builder-only bookkeeping fields (id, sourceDashboardId, emptySections, updatedAt)", () => {
    const withBookkeeping = draft({
      id: "some-other-id",
      sourceDashboardId: "live1",
      emptySections: ["Not yet populated"],
      updatedAt: "2099-01-01T00:00:00.000Z",
    });
    expect(isDraftDrifted(withBookkeeping, live())).toBe(false);
  });
});
