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

import { useEffect, useState } from "react";
import type { BeDashboardWidget } from "@api/backend/types";

/**
 * Locally designed shared dashboard config: the filter presets and reusable
 * sections a dashboard definition references by name.
 *
 * Persisted to `localStorage` only, exactly like `dashboardDraftsStorage` and
 * for exactly the same reason: these are deployment configuration files
 * (`_presets.json`, `_sections.json`) served from a directory a maintainer
 * deploys, and there is no write API for them. The designer's output is JSON
 * to hand over, not a saved record.
 *
 * Seeded from what is actually deployed (`GET /dashboards/filter-presets` and
 * `GET /dashboards/sections`) the first time the designer is opened, so
 * editing starts from reality rather than from an empty page — see
 * `seedSharedConfigDraft`.
 */

const STORAGE_KEY = "csm.dashboardSharedConfigDraft";

/** Same in-tab notification idiom as `dashboardDraftsStorage` — the native
 * `storage` event only fires for OTHER tabs. */
const STORAGE_EVENT = "csm.dashboardSharedConfigDraft.changed";

/** One designed preset: a name plus the single filter predicate it stands
 * for. The predicate is the same field/op/values object a widget's own
 * `query.filters` entries use; typed loosely because neither this app nor the
 * backend interprets it beyond forwarding it. */
export interface PresetDraft {
  name: string;
  filter: Record<string, unknown>;
}

/** One designed reusable section: a name, the heading its widgets are
 * grouped under, and the widget run itself.
 *
 * `widgets` is the AUTHORED form: a widget here may carry `{"preset": ...}`
 * references in its `query.filters`, unexpanded, because that is what gets
 * written to `_sections.json`. Never hand these queries to a `/search`
 * endpoint. */
export interface SectionDraft {
  name: string;
  displayName: string;
  widgets: BeDashboardWidget[];
}

export interface SharedConfigDraft {
  presets: PresetDraft[];
  sections: SectionDraft[];
  /** ISO timestamp of the last local save. */
  updatedAt: string;
  /** Whether the deployed catalogues have already been folded in, so a
   * later visit does not re-seed over the admin's own edits (including
   * deletions, which an "is it empty?" check could not distinguish from a
   * fresh start). */
  seeded: boolean;
}

const EMPTY: SharedConfigDraft = {
  presets: [],
  sections: [],
  updatedAt: "",
  seeded: false,
};

function read(): SharedConfigDraft {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return EMPTY;
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object") return EMPTY;
    const o = parsed as Partial<SharedConfigDraft>;
    // Every field defended individually: this is user-editable storage that
    // can also have been written by an older version of this code.
    return {
      presets: Array.isArray(o.presets) ? o.presets : [],
      sections: Array.isArray(o.sections) ? o.sections : [],
      updatedAt: typeof o.updatedAt === "string" ? o.updatedAt : "",
      seeded: o.seeded === true,
    };
  } catch {
    return EMPTY;
  }
}

function write(draft: SharedConfigDraft): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(draft));
    window.dispatchEvent(new CustomEvent(STORAGE_EVENT));
  } catch {
    // ignore quota / serialization errors — same reasoning as
    // dashboardDraftsStorage: only persistence fails, not the in-memory edit.
  }
}

export function getSharedConfigDraft(): SharedConfigDraft {
  return read();
}

/** Saves the whole draft, stamping `updatedAt`. */
export function saveSharedConfigDraft(
  draft: Omit<SharedConfigDraft, "updatedAt">,
): SharedConfigDraft {
  const stamped: SharedConfigDraft = {
    ...draft,
    updatedAt: new Date().toISOString(),
  };
  write(stamped);
  return stamped;
}

/**
 * Folds the deployed catalogues into the local draft, once.
 *
 * Deployed entries the draft does not already name are added; an entry the
 * draft already names is left alone, because the draft's version is the
 * admin's in-progress edit of it. After the first run `seeded` is set and
 * this becomes a no-op, so a deleted entry stays deleted instead of
 * reappearing on the next visit.
 */
export function seedSharedConfigDraft(
  deployedPresets: readonly PresetDraft[],
  deployedSections: readonly SectionDraft[],
): SharedConfigDraft {
  const current = read();
  if (current.seeded) return current;

  const presetNames = new Set(current.presets.map((p) => p.name));
  const sectionNames = new Set(current.sections.map((s) => s.name));

  return saveSharedConfigDraft({
    presets: [
      ...current.presets,
      ...deployedPresets.filter((p) => !presetNames.has(p.name)),
    ],
    sections: [
      ...current.sections,
      ...deployedSections.filter((s) => !sectionNames.has(s.name)),
    ],
    seeded: true,
  });
}

/** Reactive draft, kept in sync across components and browser tabs the same
 * way `useDashboardDrafts` is. */
export function useSharedConfigDraft(): SharedConfigDraft {
  const [draft, setDraft] = useState<SharedConfigDraft>(() => read());
  useEffect(() => {
    const sync = (): void => setDraft(read());
    sync();
    window.addEventListener(STORAGE_EVENT, sync);
    window.addEventListener("storage", sync);
    return () => {
      window.removeEventListener(STORAGE_EVENT, sync);
      window.removeEventListener("storage", sync);
    };
  }, []);
  return draft;
}

/**
 * The deployable `_presets.json`: a name-keyed object, which is the shape the
 * backend's own loader reads (`LoadSharedPresets`), NOT the array shape the
 * catalogue endpoint returns.
 *
 * Keys are emitted in sorted order so re-exporting an unchanged draft
 * produces an identical file and the config diff stays readable.
 */
export function presetsFileFromDraft(
  presets: readonly PresetDraft[],
): Record<string, Record<string, unknown>> {
  const out: Record<string, Record<string, unknown>> = {};
  for (const p of [...presets].sort((a, b) => a.name.localeCompare(b.name))) {
    if (p.name.trim().length === 0) continue;
    out[p.name] = p.filter;
  }
  return out;
}

/**
 * The deployable `_sections.json`: a name-keyed object of
 * `{displayName, widgets}`, again the loader's shape rather than the
 * endpoint's.
 *
 * Each widget is written back with `id` rather than the `widgetId` the API
 * returns — the definition file's key is `id`, and a section file carrying
 * `widgetId` would fail to load. That rename is the one real translation
 * between the wire shape the builder edits and the file shape it emits.
 */
export function sectionsFileFromDraft(
  sections: readonly SectionDraft[],
): Record<string, { displayName: string; widgets: Record<string, unknown>[] }> {
  const out: Record<
    string,
    { displayName: string; widgets: Record<string, unknown>[] }
  > = {};
  for (const s of [...sections].sort((a, b) => a.name.localeCompare(b.name))) {
    if (s.name.trim().length === 0) continue;
    out[s.name] = {
      displayName: s.displayName,
      widgets: s.widgets.map(widgetToDefinition),
    };
  }
  return out;
}

/**
 * One widget in definition-file shape: `widgetId` becomes `id`, and keys the
 * loader has no use for are dropped rather than emitted as nulls.
 *
 * `section` is deliberately omitted: within a section file the heading comes
 * from the section's own `displayName`, and the loader overwrites each
 * included widget's `section` with it, so carrying one here would be dead
 * config that silently disagrees with what actually renders.
 */
export function widgetToDefinition(
  widget: BeDashboardWidget,
): Record<string, unknown> {
  const out: Record<string, unknown> = {
    id: widget.widgetId,
    displayName: widget.displayName,
    resourceType: widget.resourceType,
    shape: widget.shape,
    gridWidth: widget.gridWidth,
  };
  if (widget.description) out.description = widget.description;
  if (widget.listLimit !== undefined) out.listLimit = widget.listLimit;
  if (widget.sortBy) out.sortBy = widget.sortBy;
  if (widget.columns && widget.columns.length > 0) out.columns = widget.columns;
  if (widget.groupBy) out.groupBy = widget.groupBy;
  if (widget.slices && widget.slices.length > 0) out.slices = widget.slices;
  if (widget.query) out.query = widget.query;
  return out;
}
