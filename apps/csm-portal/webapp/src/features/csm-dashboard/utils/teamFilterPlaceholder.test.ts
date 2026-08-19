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
  CURRENT_TEAM_PLACEHOLDER,
  resolveTeamPlaceholder,
} from "./teamFilterPlaceholder";

describe("resolveTeamPlaceholder", () => {
  it("substitutes the placeholder with the selected team's creGroupId when one is available", () => {
    const filters = {
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(
      filters,
      "22222222-2222-2222-2222-222222222222",
      undefined,
    );

    expect(resolved).toEqual({
      filters: [
        { field: "state", op: "in", values: ["open"] },
        {
          field: "creTeam",
          op: "in",
          values: ["22222222-2222-2222-2222-222222222222"],
        },
      ],
    });
  });

  it("substitutes the placeholder with the selected team's sreGroupId when one is available", () => {
    const filters = {
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "sreTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(
      filters,
      undefined,
      "33333333-3333-3333-3333-333333333333",
    );

    expect(resolved).toEqual({
      filters: [
        { field: "state", op: "in", values: ["open"] },
        {
          field: "sreTeam",
          op: "in",
          values: ["33333333-3333-3333-3333-333333333333"],
        },
      ],
    });
  });

  it("resolves creTeam and sreTeam entries independently on the same filters object", () => {
    const filters = {
      filters: [
        { field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
        { field: "sreTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, "cre-group-id", "sre-group-id");

    expect(resolved).toEqual({
      filters: [
        { field: "creTeam", op: "in", values: ["cre-group-id"] },
        { field: "sreTeam", op: "in", values: ["sre-group-id"] },
      ],
    });
  });

  it("drops the creTeam entry but resolves the sreTeam entry when only the cre groupId is missing", () => {
    const filters = {
      filters: [
        { field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
        { field: "sreTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, undefined, "sre-group-id");

    expect(resolved).toEqual({
      filters: [{ field: "sreTeam", op: "in", values: ["sre-group-id"] }],
    });
  });

  it("drops the creTeam entry entirely when no creGroupId is available", () => {
    const filters = {
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, undefined, undefined);

    expect(resolved).toEqual({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
  });

  it("drops the sreTeam entry entirely when no sreGroupId is available", () => {
    const filters = {
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "sreTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, undefined, undefined);

    expect(resolved).toEqual({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
  });

  it("only substitutes the placeholder entry within a values array, leaving other literal values alone", () => {
    const filters = {
      filters: [
        {
          field: "creTeam",
          op: "in",
          values: ["some-literal-group-id", CURRENT_TEAM_PLACEHOLDER],
        },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, "team-group-id", undefined);

    expect(resolved).toEqual({
      filters: [
        {
          field: "creTeam",
          op: "in",
          values: ["some-literal-group-id", "team-group-id"],
        },
      ],
    });
  });

  it("returns filters unchanged when there's no creTeam or sreTeam entry at all", () => {
    const filters = { filters: [{ field: "state", op: "in", values: ["open"] }] };

    expect(resolveTeamPlaceholder(filters, "team-group-id", "team-group-id")).toEqual(filters);
    expect(resolveTeamPlaceholder(filters, undefined, undefined)).toEqual(filters);
  });

  it("returns non-case-filter-shaped filters (other resourceTypes) unchanged", () => {
    const filters = { states: ["open"], severities: ["critical"] };

    expect(resolveTeamPlaceholder(filters, "team-group-id", "team-group-id")).toBe(filters);
  });

  it("passes through an empty filters array unchanged", () => {
    const filters = { filters: [] };

    expect(resolveTeamPlaceholder(filters, "team-group-id", "team-group-id")).toBe(filters);
  });

  it("drops the creTeam entry entirely when given an array with many ids ('All ABTs')", () => {
    const filters = {
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, ["group-a", "group-b", "group-c"], undefined);

    expect(resolved).toEqual({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
  });

  it("drops the sreTeam entry entirely when given an array with many ids ('All ABTs')", () => {
    const filters = {
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "sreTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, undefined, ["group-a", "group-b", "group-c"]);

    expect(resolved).toEqual({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
  });

  it("drops the creTeam entry entirely when given an array with a single id", () => {
    const filters = {
      filters: [
        {
          field: "creTeam",
          op: "in",
          values: ["some-literal-group-id", CURRENT_TEAM_PLACEHOLDER],
        },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, ["group-a"], undefined);

    expect(resolved).toEqual({ filters: [] });
  });

  it("drops the creTeam entry entirely when given an empty array", () => {
    const filters = {
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, [], undefined);

    expect(resolved).toEqual({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
  });

  it("still substitutes a single string 1:1, same as before array support existed", () => {
    const filters = {
      filters: [
        { field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
      ],
    };

    const resolved = resolveTeamPlaceholder(filters, "single-group-id", undefined);

    expect(resolved).toEqual({
      filters: [{ field: "creTeam", op: "in", values: ["single-group-id"] }],
    });
  });

  // Regression test: DASHBOARDS_CONFIG is a raw JSON env var, not
  // schema-validated beyond basic decoding — a widget/slice entry missing
  // `filters` entirely used to crash here with "Cannot read properties of
  // undefined (reading 'filters')" despite the wire type declaring it
  // required.
  it("treats an undefined filters argument as empty rather than throwing", () => {
    // Runtime-only case: DASHBOARDS_CONFIG is a raw JSON env var, not
    // schema-validated beyond basic decoding, so a genuinely-absent filters
    // value is possible despite the type declaring it required — forcing it
    // through here (rather than widening the signature) is what actually
    // pins the defensive runtime behavior.
    const undefinedFilters = undefined as unknown as Record<string, unknown>;
    expect(resolveTeamPlaceholder(undefinedFilters, "team-group-id", undefined)).toEqual({});
    expect(resolveTeamPlaceholder(undefinedFilters, undefined, undefined)).toEqual({});
  });
});
