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
import {
  ADVANCED_FILTER_FIELDS,
  RELATIVE_DATE_PRESETS,
  getAdvancedFilterFieldMeta,
  getAdvancedFilterOpMeta,
} from "./advancedFilters";

describe("ADVANCED_FILTER_FIELDS — duplication fix", () => {
  it("no longer offers tag, projectOnboardingStatus, or creTeam (each has its own dedicated bar control)", () => {
    const fields = ADVANCED_FILTER_FIELDS.map((m) => m.field);
    expect(fields).not.toContain("tag");
    expect(fields).not.toContain("projectOnboardingStatus");
    expect(fields).not.toContain("creTeam");
  });

  it("still offers every other previously-supported field", () => {
    const fields = ADVANCED_FILTER_FIELDS.map((m) => m.field);
    expect(fields).toEqual(
      expect.arrayContaining([
        "projectType",
        "issueType",
        "sreTeam",
        "deploymentId",
        "number",
        "internalId",
        "resolutionNotes",
        "parentId",
        "taskSLABusinessElapsedPercent",
        "escalationLevel",
        "escalation",
        "createdBy",
        "createdOn",
        "updatedOn",
        "closedOn",
      ]),
    );
  });
});

describe("ADVANCED_FILTER_FIELDS — real suggestions instead of hand-typed values", () => {
  it("sreTeam is a multiSelect (real options threaded in by the builder), not free text", () => {
    const opMeta = getAdvancedFilterOpMeta("sreTeam", "in");
    expect(opMeta?.valueKind).toBe("multiSelect");
    // Static catalogue carries no options for it — they're fetched team data,
    // supplied via `AdvancedFiltersBuilder`'s `sreTeamOptions` prop instead.
    expect(getAdvancedFilterFieldMeta("sreTeam")?.options).toBeUndefined();
  });

  it("createdBy's `in` op is an async directory search, not hand-typed emails", () => {
    const opMeta = getAdvancedFilterOpMeta("createdBy", "in");
    expect(opMeta?.valueKind).toBe("asyncEmailMultiSelect");
  });

  it("createdBy's `eq` op (\"is me\") is still the value-less currentUser kind", () => {
    const opMeta = getAdvancedFilterOpMeta("createdBy", "eq");
    expect(opMeta?.valueKind).toBe("currentUser");
  });

  it("createdOn/updatedOn/closedOn gte+lte are all the preset-or-date-picker kind, not a bare date TextField", () => {
    for (const field of ["createdOn", "updatedOn", "closedOn"] as const) {
      expect(getAdvancedFilterOpMeta(field, "gte")?.valueKind).toBe("dateOrPreset");
      expect(getAdvancedFilterOpMeta(field, "lte")?.valueKind).toBe("dateOrPreset");
    }
  });

  it("deploymentId stays free text — no deployment-search component exists to back it", () => {
    expect(getAdvancedFilterOpMeta("deploymentId", "in")?.valueKind).toBe("multiText");
  });

  it("number/internalId/parentId stay free text — genuinely arbitrary opaque identifiers", () => {
    expect(getAdvancedFilterOpMeta("number", "eq")?.valueKind).toBe("text");
    expect(getAdvancedFilterOpMeta("internalId", "eq")?.valueKind).toBe("text");
    expect(getAdvancedFilterOpMeta("parentId", "eq")?.valueKind).toBe("text");
  });

  it("taskSLABusinessElapsedPercent stays a plain number input", () => {
    expect(getAdvancedFilterOpMeta("taskSLABusinessElapsedPercent", "gte")?.valueKind).toBe(
      "number",
    );
  });
});

describe("RELATIVE_DATE_PRESETS", () => {
  it("every preset value matches the placeholder grammar the resolver recognizes", () => {
    for (const preset of RELATIVE_DATE_PRESETS) {
      expect(preset.value).toMatch(/^__[a-zA-Z]+(?::-?\d+)?__$/);
      expect(preset.label.length).toBeGreaterThan(0);
    }
  });
});
