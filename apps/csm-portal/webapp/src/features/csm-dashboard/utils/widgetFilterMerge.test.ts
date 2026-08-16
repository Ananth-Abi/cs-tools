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
import { mergeWidgetFilters } from "./widgetFilterMerge";

describe("mergeWidgetFilters", () => {
  it("merges non-case filters as a plain object spread, slice keys winning", () => {
    const base = { states: ["open"], severities: ["critical"] };
    const slice = { severities: ["catastrophic"] };

    expect(mergeWidgetFilters(base, slice)).toEqual({
      states: ["open"],
      severities: ["catastrophic"],
    });
  });

  it("merges case filter arrays by field, keeping base entries the slice doesn't override", () => {
    const base = {
      filters: [
        { field: "state", op: "in", values: ["open", "work_in_progress"] },
        { field: "severity", op: "in", values: ["critical"] },
      ],
    };
    const slice = {
      filters: [{ field: "severity", op: "in", values: ["catastrophic"] }],
    };

    expect(mergeWidgetFilters(base, slice)).toEqual({
      filters: [
        { field: "state", op: "in", values: ["open", "work_in_progress"] },
        { field: "severity", op: "in", values: ["catastrophic"] },
      ],
    });
  });

  it("keeps the base's case filters when the slice's own filters array is empty", () => {
    const base = { filters: [{ field: "state", op: "in", values: ["open"] }] };
    const slice = { filters: [] };

    expect(mergeWidgetFilters(base, slice)).toEqual({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
  });

  // Regression test: DASHBOARDS_CONFIG is a raw JSON env var, not
  // schema-validated beyond basic decoding — a widget's base `filters` or a
  // pie/bar slice's own `filters` can be genuinely absent at runtime despite
  // the wire type declaring both required. This used to crash with "Cannot
  // read properties of undefined (reading 'filters')" three calls down from
  // DashboardWidgetTile/useWidgetPieData.
  it("treats an undefined base or slice as empty rather than throwing", () => {
    expect(mergeWidgetFilters(undefined, { states: ["open"] })).toEqual({ states: ["open"] });
    expect(mergeWidgetFilters({ states: ["open"] }, undefined)).toEqual({ states: ["open"] });
    expect(mergeWidgetFilters(undefined, undefined)).toEqual({});
  });
});
