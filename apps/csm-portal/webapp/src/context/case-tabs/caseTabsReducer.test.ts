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
  caseTabsReducer,
  INITIAL_CASE_TABS_STATE,
  type CaseTabsState,
} from "@context/case-tabs/caseTabsReducer";
import { MAX_OPEN_CASE_TABS } from "@context/case-tabs/caseTabsTypes";

function open(
  state: CaseTabsState,
  id: string,
  caseId: string,
  kind: "case" | "engagement" = "case",
): CaseTabsState {
  return caseTabsReducer(state, {
    type: "OPEN_OR_ACTIVATE",
    id,
    caseId,
    kind,
    path: `/cases/${caseId}`,
  });
}

describe("caseTabsReducer", () => {
  it("opens a new tab and makes it active", () => {
    const state = open(INITIAL_CASE_TABS_STATE, "t1", "CS1");
    expect(state.tabs).toHaveLength(1);
    expect(state.tabs[0]).toMatchObject({ id: "t1", caseId: "CS1", kind: "case" });
    expect(state.activeTabId).toBe("t1");
  });

  it("activates (not duplicates) an already-open case", () => {
    let state = open(INITIAL_CASE_TABS_STATE, "t1", "CS1");
    state = open(state, "t2", "CS2");
    // OPEN_OR_ACTIVATE for CS1 again, with a different synthetic id — the
    // reducer must find it by caseId and reuse the existing tab, not add one.
    state = open(state, "t1-again-ignored", "CS1");
    expect(state.tabs).toHaveLength(2);
    expect(state.activeTabId).toBe("t1");
  });

  it("refuses to open a new tab past the cap", () => {
    let state = INITIAL_CASE_TABS_STATE;
    for (let i = 0; i < MAX_OPEN_CASE_TABS; i++) {
      state = open(state, `t${i}`, `CS${i}`);
    }
    expect(state.tabs).toHaveLength(MAX_OPEN_CASE_TABS);
    const blocked = open(state, "overflow", "CS-overflow");
    expect(blocked.tabs).toHaveLength(MAX_OPEN_CASE_TABS);
    expect(blocked.tabs.some((t) => t.caseId === "CS-overflow")).toBe(false);
  });

  it("still activates an already-open tab even when at the cap", () => {
    let state = INITIAL_CASE_TABS_STATE;
    for (let i = 0; i < MAX_OPEN_CASE_TABS; i++) {
      state = open(state, `t${i}`, `CS${i}`);
    }
    state = open(state, "ignored", "CS2");
    expect(state.activeTabId).toBe("t2");
  });

  it("closes a tab and activates its right neighbor", () => {
    let state = INITIAL_CASE_TABS_STATE;
    state = open(state, "t1", "CS1");
    state = open(state, "t2", "CS2");
    state = open(state, "t3", "CS3");
    state = caseTabsReducer(state, { type: "SET_ACTIVE", id: "t2" });
    state = caseTabsReducer(state, { type: "CLOSE", id: "t2" });
    expect(state.tabs.map((t) => t.id)).toEqual(["t1", "t3"]);
    expect(state.activeTabId).toBe("t3");
  });

  it("closes the last tab and falls back to its left neighbor", () => {
    let state = INITIAL_CASE_TABS_STATE;
    state = open(state, "t1", "CS1");
    state = open(state, "t2", "CS2");
    state = caseTabsReducer(state, { type: "CLOSE", id: "t2" });
    expect(state.tabs.map((t) => t.id)).toEqual(["t1"]);
    expect(state.activeTabId).toBe("t1");
  });

  it("closing the only tab leaves no active tab", () => {
    let state = open(INITIAL_CASE_TABS_STATE, "t1", "CS1");
    state = caseTabsReducer(state, { type: "CLOSE", id: "t1" });
    expect(state.tabs).toHaveLength(0);
    expect(state.activeTabId).toBeNull();
  });

  it("closing a background (non-active) tab leaves the active tab untouched", () => {
    let state = INITIAL_CASE_TABS_STATE;
    state = open(state, "t1", "CS1");
    state = open(state, "t2", "CS2"); // t2 becomes active
    state = caseTabsReducer(state, { type: "CLOSE", id: "t1" });
    expect(state.activeTabId).toBe("t2");
    expect(state.tabs.map((t) => t.id)).toEqual(["t2"]);
  });

  it("updates a tab's path/kind in place without changing its id or position", () => {
    let state = INITIAL_CASE_TABS_STATE;
    state = open(state, "t1", "CS1");
    state = open(state, "t2", "CS2");
    state = caseTabsReducer(state, {
      type: "UPDATE_TAB_PATH",
      id: "t1",
      kind: "engagement",
      path: "/engagements/CS1",
    });
    expect(state.tabs.map((t) => t.id)).toEqual(["t1", "t2"]);
    expect(state.tabs[0]).toMatchObject({
      id: "t1",
      caseId: "CS1",
      kind: "engagement",
      path: "/engagements/CS1",
    });
  });

  it("sets a tab's label and draft flag independently", () => {
    let state = open(INITIAL_CASE_TABS_STATE, "t1", "CS1");
    state = caseTabsReducer(state, { type: "SET_LABEL", id: "t1", label: "CS1 · Subject" });
    state = caseTabsReducer(state, { type: "SET_DRAFT", id: "t1", hasDraft: true });
    expect(state.tabs[0].label).toBe("CS1 · Subject");
    expect(state.tabs[0].hasDraft).toBe(true);
  });

  it("hydrates from a persisted state wholesale", () => {
    const persisted: CaseTabsState = {
      tabs: [{ id: "t1", caseId: "CS1", kind: "case", path: "/cases/CS1", hasDraft: false }],
      activeTabId: "t1",
    };
    const state = caseTabsReducer(INITIAL_CASE_TABS_STATE, {
      type: "HYDRATE",
      state: persisted,
    });
    expect(state).toEqual(persisted);
  });

  it("is a no-op for an action targeting an unknown tab id", () => {
    const state = open(INITIAL_CASE_TABS_STATE, "t1", "CS1");
    const unchanged = caseTabsReducer(state, { type: "SET_ACTIVE", id: "does-not-exist" });
    expect(unchanged).toBe(state);
    const unchanged2 = caseTabsReducer(state, { type: "CLOSE", id: "does-not-exist" });
    expect(unchanged2).toBe(state);
  });
});
