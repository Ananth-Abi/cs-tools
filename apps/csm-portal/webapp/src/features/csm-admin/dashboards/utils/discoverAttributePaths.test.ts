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
import { discoverAttributePaths } from "@features/csm-admin/dashboards/utils/discoverAttributePaths";

describe("discoverAttributePaths", () => {
  it("returns top-level scalar paths", () => {
    expect(discoverAttributePaths([{ id: "CS-1", state: "open" }])).toEqual(["id", "state"]);
  });

  it("walks nested objects into dot-separated paths", () => {
    expect(
      discoverAttributePaths([
        { id: "CS-1", project: { key: "ABC", name: "Foo" } },
      ]),
    ).toEqual(["id", "project.key", "project.name"]);
  });

  it("walks arbitrarily deep nesting, up to the depth cap", () => {
    expect(
      discoverAttributePaths([{ project: { account: { tier: "gold" } } }]),
    ).toEqual(["project.account.tier"]);
  });

  it("treats an array of scalars as a leaf path, not indexed into", () => {
    expect(discoverAttributePaths([{ id: "CS-1", tags: ["a", "b"] }])).toEqual(["id", "tags"]);
  });

  it("treats an array of objects as a leaf path too, not indexed into", () => {
    expect(
      discoverAttributePaths([
        { id: "CS-1", comments: [{ id: "c-1", body: "hi" }, { id: "c-2", body: "there" }] },
      ]),
    ).toEqual(["comments", "id"]);
  });

  it("treats an empty array the same as a non-empty one — a leaf path, not dropped", () => {
    expect(discoverAttributePaths([{ id: "CS-1", tags: [] }])).toEqual(["id", "tags"]);
  });

  it("treats null and undefined leaves as valid, still-offered paths", () => {
    expect(
      discoverAttributePaths([{ id: "CS-1", closedAt: null, resolvedBy: undefined }]),
    ).toEqual(["closedAt", "id", "resolvedBy"]);
  });

  it("treats an empty object as a leaf rather than dropping its path", () => {
    expect(discoverAttributePaths([{ id: "CS-1", metadata: {} }])).toEqual(["id", "metadata"]);
  });

  it("unions paths across sampled rows with varying shapes, not just the first row", () => {
    const rows = [
      { id: "CS-1", project: { key: "ABC" } },
      { id: "CS-2", assignee: { name: "Jane Doe" } },
      { id: "CS-3", project: { key: "DEF", name: "Foo" } },
    ];
    expect(discoverAttributePaths(rows)).toEqual([
      "assignee.name",
      "id",
      "project.key",
      "project.name",
    ]);
  });

  it("deduplicates a path that recurs across multiple sampled rows", () => {
    const rows = [{ id: "CS-1" }, { id: "CS-2" }, { id: "CS-3" }];
    expect(discoverAttributePaths(rows)).toEqual(["id"]);
  });

  it("samples at most the first 20 rows, ignoring a field only present beyond that", () => {
    const rows = Array.from({ length: 25 }, (_, i) => ({ id: `CS-${i}` }));
    rows[24] = { id: "CS-24", onlyOnLastRow: "x" } as unknown as { id: string };
    expect(discoverAttributePaths(rows)).toEqual(["id"]);
  });

  it("still picks up a field present within the first 20 rows even if absent from row 0", () => {
    const rows = Array.from({ length: 25 }, (_, i) => ({ id: `CS-${i}` }));
    rows[5] = { id: "CS-5", onlyOnRow5: "x" } as unknown as { id: string; onlyOnRow5?: string };
    expect(discoverAttributePaths(rows)).toEqual(["id", "onlyOnRow5"]);
  });

  it("caps recursion depth rather than looping forever on a self-referential structure", () => {
    const cyclic: Record<string, unknown> = { id: "CS-1" };
    cyclic.self = cyclic;
    expect(() => discoverAttributePaths([cyclic])).not.toThrow();
    const result = discoverAttributePaths([cyclic]);
    expect(result).toContain("id");
    expect(result.some((p) => p.startsWith("self."))).toBe(true);
    // The deepest path stops growing once the cap is hit — 6 "self" hops
    // then it's cut off as a leaf, not an unbounded "self.self.self...".
    const deepest = result.filter((p) => p.startsWith("self.")).sort().pop();
    expect(deepest?.split(".").length).toBeLessThanOrEqual(6);
  });

  it("returns an empty list for an empty rows array", () => {
    expect(discoverAttributePaths([])).toEqual([]);
  });

  it("skips a non-object row rather than throwing", () => {
    expect(discoverAttributePaths([null as unknown as Record<string, unknown>])).toEqual([]);
  });
});
