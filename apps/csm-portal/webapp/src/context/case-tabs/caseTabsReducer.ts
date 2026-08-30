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

import {
  MAX_OPEN_CASE_TABS,
  type CaseRouteKind,
  type CaseTabState,
} from "@context/case-tabs/caseTabsTypes";

export interface CaseTabsState {
  tabs: CaseTabState[];
  activeTabId: string | null;
}

export const INITIAL_CASE_TABS_STATE: CaseTabsState = {
  tabs: [],
  activeTabId: null,
};

export type CaseTabsAction =
  | {
      type: "OPEN_OR_ACTIVATE";
      id: string;
      caseId: string;
      kind: CaseRouteKind;
      path: string;
      state?: unknown;
    }
  | { type: "CLOSE"; id: string }
  | { type: "SET_ACTIVE"; id: string }
  /** Same case, path (and/or kind) changed in place — see
   * `CaseTabIsolatedRouter`'s navigator. */
  | { type: "UPDATE_TAB_PATH"; id: string; kind: CaseRouteKind; path: string }
  | { type: "SET_LABEL"; id: string; label: string | undefined }
  | { type: "SET_DRAFT"; id: string; hasDraft: boolean }
  | { type: "HYDRATE"; state: CaseTabsState };

/** Picks the tab that should become active after the given one closes: the
 * next tab to its right, or failing that the previous one, matching the
 * common browser-tab-strip convention. `tabsBeforeClose` still contains the
 * closed tab at `closedIndex`. */
function nextActiveAfterClose(
  tabsBeforeClose: CaseTabState[],
  closedIndex: number,
): string | null {
  const rightNeighbor = tabsBeforeClose[closedIndex + 1];
  if (rightNeighbor) return rightNeighbor.id;
  const leftNeighbor = tabsBeforeClose[closedIndex - 1];
  return leftNeighbor?.id ?? null;
}

export function caseTabsReducer(
  state: CaseTabsState,
  action: CaseTabsAction,
): CaseTabsState {
  switch (action.type) {
    case "HYDRATE":
      return action.state;

    case "OPEN_OR_ACTIVATE": {
      const existing = state.tabs.find((t) => t.caseId === action.caseId);
      if (existing) {
        return existing.id === state.activeTabId
          ? state
          : { ...state, activeTabId: existing.id };
      }
      if (state.tabs.length >= MAX_OPEN_CASE_TABS) {
        // Caller (useCaseTabsController.openTab) is responsible for checking
        // capacity BEFORE dispatching and surfacing the "close a tab first"
        // message; this is a defensive no-op if it's ever dispatched anyway.
        return state;
      }
      const newTab: CaseTabState = {
        id: action.id,
        caseId: action.caseId,
        kind: action.kind,
        path: action.path,
        hasDraft: false,
        state: action.state,
      };
      return {
        tabs: [...state.tabs, newTab],
        activeTabId: newTab.id,
      };
    }

    case "CLOSE": {
      const index = state.tabs.findIndex((t) => t.id === action.id);
      if (index === -1) return state;
      const tabs = state.tabs.filter((t) => t.id !== action.id);
      const activeTabId =
        state.activeTabId === action.id
          ? nextActiveAfterClose(state.tabs, index)
          : state.activeTabId;
      return { tabs, activeTabId };
    }

    case "SET_ACTIVE": {
      if (state.activeTabId === action.id) return state;
      if (!state.tabs.some((t) => t.id === action.id)) return state;
      return { ...state, activeTabId: action.id };
    }

    case "UPDATE_TAB_PATH": {
      const index = state.tabs.findIndex((t) => t.id === action.id);
      if (index === -1) return state;
      const current = state.tabs[index];
      if (current.path === action.path && current.kind === action.kind) {
        return state;
      }
      const tabs = state.tabs.slice();
      tabs[index] = { ...current, path: action.path, kind: action.kind };
      return { ...state, tabs };
    }

    case "SET_LABEL": {
      const index = state.tabs.findIndex((t) => t.id === action.id);
      if (index === -1 || state.tabs[index].label === action.label) return state;
      const tabs = state.tabs.slice();
      tabs[index] = { ...tabs[index], label: action.label };
      return { ...state, tabs };
    }

    case "SET_DRAFT": {
      const index = state.tabs.findIndex((t) => t.id === action.id);
      if (index === -1 || state.tabs[index].hasDraft === action.hasDraft) {
        return state;
      }
      const tabs = state.tabs.slice();
      tabs[index] = { ...tabs[index], hasDraft: action.hasDraft };
      return { ...state, tabs };
    }

    default:
      return state;
  }
}
