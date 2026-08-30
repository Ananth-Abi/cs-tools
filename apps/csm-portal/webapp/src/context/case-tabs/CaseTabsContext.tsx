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

/* eslint-disable react-refresh/only-export-components -- Provider component and its useXxx hook are colocated per the repo's context idiom (fast-refresh DX only) */

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  type JSX,
  type ReactNode,
} from "react";
import {
  caseTabsReducer,
  INITIAL_CASE_TABS_STATE,
  type CaseTabsState,
} from "@context/case-tabs/caseTabsReducer";
import {
  MAX_OPEN_CASE_TABS,
  type CaseRouteKind,
  type CaseTabState,
  type CaseTabsPersistedState,
} from "@context/case-tabs/caseTabsTypes";

// Deliberately sessionStorage, not localStorage: an open-tabs list is
// per-browser-session working state, not something that should survive
// across logins/users on a shared machine (see this feature's design notes).
const STORAGE_KEY = "csm.caseTabs.v1";

function readPersistedState(): CaseTabsState | undefined {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return undefined;
    const parsed = JSON.parse(raw) as CaseTabsPersistedState;
    if (!Array.isArray(parsed.tabs)) return undefined;
    const tabs: CaseTabState[] = parsed.tabs.map((t) => ({
      ...t,
      hasDraft: false,
      label: undefined,
    }));
    const activeTabId = tabs.some((t) => t.id === parsed.activeTabId)
      ? parsed.activeTabId
      : (tabs[tabs.length - 1]?.id ?? null);
    return { tabs, activeTabId };
  } catch {
    return undefined;
  }
}

function writePersistedState(state: CaseTabsState): void {
  try {
    const persisted: CaseTabsPersistedState = {
      tabs: state.tabs.map(({ id, caseId, kind, path }) => ({
        id,
        caseId,
        kind,
        path,
      })),
      activeTabId: state.activeTabId,
    };
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(persisted));
  } catch {
    /* sessionStorage unavailable (private mode, quota, ...) — tabs still
     * work for the current page-load, they just won't survive a reload. */
  }
}

let tabIdCounter = 0;
function nextTabId(): string {
  tabIdCounter += 1;
  return `case-tab-${Date.now().toString(36)}-${tabIdCounter}`;
}

export interface CaseTabsController {
  tabs: CaseTabState[];
  activeTabId: string | null;
  activeTab: CaseTabState | undefined;
  /** Opens a tab for `caseId` (or activates it if already open). Returns
   * `false` without changing state when the cap is already reached and this
   * would be a genuinely new tab — the caller (list row click, route sync)
   * decides how to surface that (a toast, or simply rendering the record
   * without a tab). */
  openTab: (caseId: string, kind: CaseRouteKind, path: string) => boolean;
  closeTab: (id: string) => void;
  setActiveTab: (id: string) => void;
  updateTabPath: (id: string, kind: CaseRouteKind, path: string) => void;
  setTabLabel: (id: string, label: string | undefined) => void;
  setTabDraft: (id: string, hasDraft: boolean) => void;
  isAtCapacity: boolean;
}

// A no-op default (not `undefined`) so `CsmCaseDetailPage` — which now
// unconditionally calls `useCaseTabsController` (via
// `useReportCaseTabDraft`) — keeps working exactly as it did before this
// feature existed when rendered outside a `CaseTabsProvider`: its many
// existing tests, Storybook-style isolated renders, etc. None of them
// exercise tab behavior, so a silent no-op is correct here, not just
// convenient — there is no "tab" for a page rendered standalone to report
// into.
const NOOP_CASE_TABS_CONTROLLER: CaseTabsController = {
  tabs: [],
  activeTabId: null,
  activeTab: undefined,
  openTab: () => true,
  closeTab: () => {},
  setActiveTab: () => {},
  updateTabPath: () => {},
  setTabLabel: () => {},
  setTabDraft: () => {},
  isAtCapacity: false,
};

const CaseTabsContext = createContext<CaseTabsController>(NOOP_CASE_TABS_CONTROLLER);

export function CaseTabsProvider({ children }: { children: ReactNode }): JSX.Element {
  const [state, dispatch] = useReducer(
    caseTabsReducer,
    undefined,
    () => readPersistedState() ?? INITIAL_CASE_TABS_STATE,
  );

  useEffect(() => {
    writePersistedState(state);
  }, [state]);

  const openTab = useCallback(
    (caseId: string, kind: CaseRouteKind, path: string): boolean => {
      const alreadyOpen = state.tabs.some((t) => t.caseId === caseId);
      if (!alreadyOpen && state.tabs.length >= MAX_OPEN_CASE_TABS) {
        return false;
      }
      dispatch({ type: "OPEN_OR_ACTIVATE", id: nextTabId(), caseId, kind, path });
      return true;
    },
    [state.tabs],
  );

  const closeTab = useCallback((id: string) => {
    dispatch({ type: "CLOSE", id });
  }, []);

  const setActiveTab = useCallback((id: string) => {
    dispatch({ type: "SET_ACTIVE", id });
  }, []);

  const updateTabPath = useCallback((id: string, kind: CaseRouteKind, path: string) => {
    dispatch({ type: "UPDATE_TAB_PATH", id, kind, path });
  }, []);

  const setTabLabel = useCallback((id: string, label: string | undefined) => {
    dispatch({ type: "SET_LABEL", id, label });
  }, []);

  const setTabDraft = useCallback((id: string, hasDraft: boolean) => {
    dispatch({ type: "SET_DRAFT", id, hasDraft });
  }, []);

  const activeTab = state.tabs.find((t) => t.id === state.activeTabId);

  const value = useMemo<CaseTabsController>(
    () => ({
      tabs: state.tabs,
      activeTabId: state.activeTabId,
      activeTab,
      openTab,
      closeTab,
      setActiveTab,
      updateTabPath,
      setTabLabel,
      setTabDraft,
      isAtCapacity: state.tabs.length >= MAX_OPEN_CASE_TABS,
    }),
    [
      state.tabs,
      state.activeTabId,
      activeTab,
      openTab,
      closeTab,
      setActiveTab,
      updateTabPath,
      setTabLabel,
      setTabDraft,
    ],
  );

  return <CaseTabsContext.Provider value={value}>{children}</CaseTabsContext.Provider>;
}

export function useCaseTabsController(): CaseTabsController {
  return useContext(CaseTabsContext);
}

/** Ref-based escape hatch for call sites that need the latest controller
 * inside a callback without adding it to a dependency array (e.g. the
 * isolated router's navigator, which is memoized once per tab instance). */
export function useCaseTabsControllerRef() {
  const controller = useCaseTabsController();
  const ref = useRef(controller);
  // Kept fresh in an effect (not written during render) — the ref is only
  // ever read later, from event/navigation callbacks, never during render.
  useEffect(() => {
    ref.current = controller;
  });
  return ref;
}
