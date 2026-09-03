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

/** Only the first this-many rows of a preview response are walked — a
 * widget's search response can carry many rows, and unioning paths across
 * every one of them buys nothing beyond a small sample (optional fields
 * either show up in the first handful of rows or they don't), so this caps
 * the work rather than walking a potentially large `items` array on every
 * Preview click. */
const MAX_SAMPLE_ROWS = 20;

/** Recursion depth cap, counted in nested-object hops from a sampled row's
 * own root. A real search-response item is a handful of levels deep at
 * most (e.g. `project.account.tier`) — this exists purely as a backstop
 * against an unexpectedly deep or (in principle, since this walks
 * `Record<string, unknown>` data of unknown provenance) self-referential
 * structure, not because 6 levels is itself meaningful. */
const MAX_DEPTH = 6;

/**
 * Walks one sampled row and adds every dot-separated path reachable in it
 * to `paths`, matching exactly what `resolveColumnPath`
 * (`features/csm-dashboard/utils/resolveWidgetColumn.ts`) can resolve a
 * widget column `path` against:
 *
 * - A plain object is walked into, one path segment per key, recursively.
 * - An array is treated as a leaf, not indexed into — `resolveColumnPath`
 *   walks a path by object-key segment only, and offering a
 *   numeric-index path (`tags.0`) would be a path meaningful for exactly
 *   one row's array length, not a column definition an admin could reuse
 *   across every row. The path up to (and including) the array itself is
 *   still offered, since it's what a column path could target.
 * - `null`/`undefined`/a scalar (string, number, boolean) is a leaf.
 * - An empty object (no keys) is treated as a leaf too — there's nothing
 *   further to union in from it, and dropping the path outright would
 *   make a real field that happens to be empty in every sampled row
 *   silently disappear from the offered list.
 * - Recursion stops at `MAX_DEPTH`, treating whatever's at that depth as a
 *   leaf, rather than continuing indefinitely (see `MAX_DEPTH`'s own doc
 *   comment).
 */
function collectPaths(value: unknown, prefix: string, depth: number, paths: Set<string>): void {
  if (Array.isArray(value)) {
    if (prefix) paths.add(prefix);
    return;
  }
  if (value !== null && typeof value === "object") {
    const entries = Object.entries(value as Record<string, unknown>);
    if (entries.length === 0 || depth >= MAX_DEPTH) {
      if (prefix) paths.add(prefix);
      return;
    }
    for (const [key, child] of entries) {
      collectPaths(child, prefix ? `${prefix}.${key}` : key, depth + 1, paths);
    }
    return;
  }
  // Scalar, null, or undefined.
  if (prefix) paths.add(prefix);
}

/**
 * Discovers the real dot-separated attribute paths reachable in a widget's
 * own Preview data, for the "Columns" editor's path autocomplete
 * (`WidgetEditorDialog`) — so an admin picks a path that's actually present
 * on this resourceType's search response instead of guessing/typing one
 * blind.
 *
 * Samples up to `MAX_SAMPLE_ROWS` rows and unions the paths found across all
 * of them (not just the first), since two rows of the same resourceType can
 * have different optional fields present/absent — a path only the third
 * sampled row happens to carry would otherwise never surface. Returns a
 * deduplicated, alphabetically sorted list.
 */
export function discoverAttributePaths(rows: Record<string, unknown>[]): string[] {
  const paths = new Set<string>();
  for (const row of rows.slice(0, MAX_SAMPLE_ROWS)) {
    if (row === null || typeof row !== "object") continue;
    collectPaths(row, "", 0, paths);
  }
  return Array.from(paths).sort();
}
