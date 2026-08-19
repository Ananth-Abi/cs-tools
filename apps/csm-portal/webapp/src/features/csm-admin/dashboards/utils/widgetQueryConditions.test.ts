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
  filterConditionsFromQuery,
  operatorsForResourceType,
  queryFromFilterConditions,
  usesCaseFieldFilterDsl,
} from "@features/csm-admin/dashboards/utils/widgetQueryConditions";

describe("usesCaseFieldFilterDsl", () => {
  it("is true for case and every case-type-variant resourceType", () => {
    for (const rt of [
      "case",
      "service_request",
      "security_report_analysis",
      "announcement",
      "engagement",
    ] as const) {
      expect(usesCaseFieldFilterDsl(rt)).toBe(true);
    }
  });

  it("is false for a non-case resourceType", () => {
    expect(usesCaseFieldFilterDsl("incident")).toBe(false);
    expect(usesCaseFieldFilterDsl("account")).toBe(false);
  });
});

describe("filterConditionsFromQuery / queryFromFilterConditions — case-like resourceType", () => {
  it("round-trips the case field/op/values DSL", () => {
    const query = {
      filters: [
        { field: "severity", op: "in", values: ["critical", "high"] },
        { field: "state", op: "in", values: ["open"] },
      ],
    };
    const conditions = filterConditionsFromQuery("case", query);
    expect(conditions).toEqual([
      { field: "severity", op: "in", values: ["critical", "high"] },
      { field: "state", op: "in", values: ["open"] },
    ]);
    expect(queryFromFilterConditions("case", conditions)).toEqual(query);
  });

  it("drops values for a value-less op (isEmpty/isNotEmpty) on serialize", () => {
    const conditions = [{ field: "escalation", op: "isNotEmpty" as const, values: [] }];
    expect(queryFromFilterConditions("case", conditions)).toEqual({
      filters: [{ field: "escalation", op: "isNotEmpty" }],
    });
  });

  it("returns an empty query for zero conditions, not an empty filters array", () => {
    expect(queryFromFilterConditions("case", [])).toEqual({});
  });

  it("returns no conditions for a query with no filters array", () => {
    expect(filterConditionsFromQuery("case", {})).toEqual([]);
    expect(filterConditionsFromQuery("case", undefined)).toEqual([]);
  });

  it("skips a malformed entry (no field) rather than crashing", () => {
    const query = { filters: [{ op: "in", values: ["x"] }, { field: "state", op: "eq" }] };
    expect(filterConditionsFromQuery("case", query)).toEqual([
      { field: "state", op: "eq", values: [] },
    ]);
  });

  it("drops a row with an empty field on serialize", () => {
    const conditions = [
      { field: "", op: "eq" as const, values: ["x"] },
      { field: "state", op: "eq" as const, values: ["open"] },
    ];
    expect(queryFromFilterConditions("case", conditions)).toEqual({
      filters: [{ field: "state", op: "eq", values: ["open"] }],
    });
  });
});

describe("filterConditionsFromQuery / queryFromFilterConditions — non-case resourceType", () => {
  it("reads a flat, non-DSL query into one row per key", () => {
    const query = { priorities: ["HIGH", "CRITICAL"], slaViolated: true, number: "INC0001" };
    const conditions = filterConditionsFromQuery("incident", query);
    expect(conditions).toEqual(
      expect.arrayContaining([
        { field: "priorities", op: "in", values: ["HIGH", "CRITICAL"] },
        { field: "slaViolated", op: "eq", values: ["true"] },
        { field: "number", op: "eq", values: ["INC0001"] },
      ]),
    );
    expect(conditions).toHaveLength(3);
  });

  it("serializes back to flat top-level keys, array for 'in', scalar otherwise", () => {
    const conditions = [
      { field: "priorities", op: "in" as const, values: ["HIGH", "CRITICAL"] },
      { field: "number", op: "eq" as const, values: ["INC0001"] },
    ];
    expect(queryFromFilterConditions("incident", conditions)).toEqual({
      priorities: ["HIGH", "CRITICAL"],
      number: "INC0001",
    });
  });

  it("never nests a non-case resourceType's query under 'filters'", () => {
    const conditions = [{ field: "priorities", op: "in" as const, values: ["HIGH"] }];
    const query = queryFromFilterConditions("incident", conditions);
    expect(query).not.toHaveProperty("filters");
  });

  it("serializes a boolean-looking eq value as a real boolean, not the string 'true'/'false'", () => {
    const conditions = [{ field: "slaViolated", op: "eq" as const, values: ["true"] }];
    expect(queryFromFilterConditions("incident", conditions)).toEqual({ slaViolated: true });
    expect(
      queryFromFilterConditions("incident", [
        { field: "slaViolated", op: "eq" as const, values: ["false"] },
      ]),
    ).toEqual({ slaViolated: false });
  });

  it("serializes a numeric-looking eq value as a real number, not a string", () => {
    const conditions = [{ field: "someCount", op: "eq" as const, values: ["42"] }];
    expect(queryFromFilterConditions("incident", conditions)).toEqual({ someCount: 42 });
  });

  it("leaves a non-numeric, non-boolean value as a plain string (e.g. a case/incident number)", () => {
    const conditions = [{ field: "number", op: "eq" as const, values: ["INC0001"] }];
    expect(queryFromFilterConditions("incident", conditions)).toEqual({ number: "INC0001" });
  });

  it("type-recovers each element of an 'in' array too", () => {
    const conditions = [{ field: "flags", op: "in" as const, values: ["true", "1", "no"] }];
    expect(queryFromFilterConditions("incident", conditions)).toEqual({
      flags: [true, 1, "no"],
    });
  });

  it("drops a row whose op the flat non-case contract can't express (legacy/hand-edited data), rather than silently reinterpreting it as eq", () => {
    // The flat contract has no per-field op of its own — `notIn` here has
    // no way to be written as "not X" and must NOT be silently rewritten
    // to `eq` ("is X"), which would flip its real meaning the moment the
    // admin saves without ever touching this row.
    const conditions = [
      { field: "slaViolated", op: "notIn" as const, values: ["true", "false"] },
      { field: "number", op: "eq" as const, values: ["INC0001"] },
    ];
    expect(queryFromFilterConditions("incident", conditions)).toEqual({ number: "INC0001" });
  });

  it("preserves a leading-zero identifier as a string, rather than silently dropping the leading zeros", () => {
    const conditions = [{ field: "number", op: "eq" as const, values: ["0090472"] }];
    expect(queryFromFilterConditions("incident", conditions)).toEqual({ number: "0090472" });
  });

  it("preserves a value above Number.MAX_SAFE_INTEGER as a string, rather than silently losing precision", () => {
    const huge = "99999999999999999999";
    const conditions = [{ field: "number", op: "eq" as const, values: [huge] }];
    expect(queryFromFilterConditions("incident", conditions)).toEqual({ number: huge });
  });
});

describe("operatorsForResourceType", () => {
  it("offers every op for a case-like resourceType", () => {
    expect(operatorsForResourceType("case")).toEqual([
      "eq",
      "in",
      "notIn",
      "gte",
      "lte",
      "isEmpty",
      "isNotEmpty",
    ]);
  });

  it("offers only eq/in for a non-case resourceType, since no other op has a real, proven query shape", () => {
    expect(operatorsForResourceType("incident")).toEqual(["eq", "in"]);
    expect(operatorsForResourceType("account")).toEqual(["eq", "in"]);
  });
});
