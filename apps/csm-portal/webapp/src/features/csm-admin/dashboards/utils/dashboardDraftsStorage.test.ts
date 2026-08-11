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

import { renderHook, act } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import {
  deleteDashboardDraft,
  getDashboardDraft,
  listDashboardDrafts,
  newDraftId,
  newWidgetId,
  saveDashboardDraft,
  useDashboardDraft,
  useDashboardDrafts,
} from "@features/csm-admin/dashboards/utils/dashboardDraftsStorage";

const BASE = {
  displayName: "Engineer overview",
  isDefault: false,
  isTeamBased: false,
  widgets: [],
  emptySections: [],
};

describe("dashboardDraftsStorage", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("returns undefined for a draft that was never saved", () => {
    expect(getDashboardDraft("nope")).toBeUndefined();
  });

  it("saves and reads back a draft, stamping updatedAt", () => {
    saveDashboardDraft({ id: "d1", ...BASE });
    const read = getDashboardDraft("d1");
    expect(read?.id).toBe("d1");
    expect(read?.displayName).toBe("Engineer overview");
    expect(typeof read?.updatedAt).toBe("string");
  });

  it("overwrites a draft saved again under the same id", () => {
    saveDashboardDraft({ id: "d1", ...BASE, displayName: "First" });
    saveDashboardDraft({ id: "d1", ...BASE, displayName: "Second" });
    expect(getDashboardDraft("d1")?.displayName).toBe("Second");
    expect(listDashboardDrafts()).toHaveLength(1);
  });

  it("deletes a draft", () => {
    saveDashboardDraft({ id: "d1", ...BASE });
    deleteDashboardDraft("d1");
    expect(getDashboardDraft("d1")).toBeUndefined();
  });

  it("newDraftId and newWidgetId never collide across calls", () => {
    const ids = new Set(Array.from({ length: 20 }, () => newDraftId()));
    expect(ids.size).toBe(20);
    const widgetIds = new Set(Array.from({ length: 20 }, () => newWidgetId()));
    expect(widgetIds.size).toBe(20);
  });

  it("lists drafts most-recently-updated first", async () => {
    saveDashboardDraft({ id: "older", ...BASE });
    await new Promise((r) => setTimeout(r, 2));
    saveDashboardDraft({ id: "newer", ...BASE });
    const ids = listDashboardDrafts().map((d) => d.id);
    expect(ids[0]).toBe("newer");
    expect(ids[1]).toBe("older");
  });

  it("useDashboardDrafts reacts to a save made elsewhere", () => {
    const { result } = renderHook(() => useDashboardDrafts());
    expect(result.current).toHaveLength(0);

    act(() => {
      saveDashboardDraft({ id: "d1", ...BASE });
    });

    expect(result.current).toHaveLength(1);
    expect(result.current[0].id).toBe("d1");
  });

  it("useDashboardDraft reacts to a save of its own id, ignoring saves of other ids", () => {
    const { result } = renderHook(() => useDashboardDraft("d1"));
    expect(result.current).toBeUndefined();

    act(() => {
      saveDashboardDraft({ id: "other", ...BASE });
    });
    expect(result.current).toBeUndefined();

    act(() => {
      saveDashboardDraft({ id: "d1", ...BASE, displayName: "Mine" });
    });
    expect(result.current?.displayName).toBe("Mine");
  });

  it("ignores corrupt JSON in localStorage rather than throwing", () => {
    localStorage.setItem("csm.dashboardBuilder.drafts.v1", "{not json");
    expect(listDashboardDrafts()).toEqual([]);
    expect(() => saveDashboardDraft({ id: "d1", ...BASE })).not.toThrow();
  });

  it("silently ignores a syntactically-valid-JSON but incomplete stored draft, rather than crashing the sort", () => {
    // A record missing `updatedAt` (e.g. hand-edited, or written by an
    // older/newer shape of this feature) used to pass the old `isDraft`
    // check, then crash `listDashboardDrafts`'s own
    // `a.updatedAt.localeCompare(b.updatedAt)` sort by calling
    // `.localeCompare` on `undefined`.
    localStorage.setItem(
      "csm.dashboardBuilder.drafts.v1",
      JSON.stringify({
        incomplete: {
          id: "incomplete",
          displayName: "Missing required fields",
          widgets: [],
          // isDefault, isTeamBased, emptySections, updatedAt all absent.
        },
        d1: { id: "d1", ...BASE, updatedAt: "2026-08-11T00:00:00.000Z" },
      }),
    );

    expect(() => listDashboardDrafts()).not.toThrow();
    const ids = listDashboardDrafts().map((d) => d.id);
    expect(ids).toEqual(["d1"]);
    expect(getDashboardDraft("incomplete")).toBeUndefined();
  });
});
