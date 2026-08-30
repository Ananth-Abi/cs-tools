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
  useMemo,
  useState,
  type JSX,
  type ReactNode,
} from "react";

const STORAGE_KEY = "csm.caseTabs.behavior";

/**
 * What happens when a distinct new case/incident/change-request is opened
 * while `MAX_OPEN_CASE_TABS` are already open:
 *  - `off` — the in-app tab mechanism (tab strip, keep-alive pages, the
 *    pinned "current location" tab) is disabled entirely; the app behaves
 *    exactly as it did before this feature existed (plain full-page
 *    navigation, one record at a time). This is the DEFAULT — the feature
 *    is beta, opt-in only.
 *  - `block` — the new one is shown standalone (no tab), with a toast; the
 *    existing 5 tabs are untouched. What the feature originally shipped
 *    with.
 *  - `evict-oldest` — the longest-open tab (first opened, not
 *    least-recently-viewed) closes to make room; the new one opens and
 *    becomes active.
 *  - `evict-newest` — the most-recently-opened tab closes instead.
 */
export type CaseTabsBehaviorMode = "off" | "block" | "evict-oldest" | "evict-newest";

export const CASE_TABS_BEHAVIOR_OPTIONS: { mode: CaseTabsBehaviorMode; label: string }[] = [
  { mode: "off", label: "No tabs (single page at a time)" },
  { mode: "block", label: "Block new tabs at the limit" },
  { mode: "evict-oldest", label: "Replace the oldest tab at the limit" },
  { mode: "evict-newest", label: "Replace the newest tab at the limit" },
];

const DEFAULT_MODE: CaseTabsBehaviorMode = "off";

function isBehaviorMode(value: unknown): value is CaseTabsBehaviorMode {
  return (
    value === "off" || value === "block" || value === "evict-oldest" || value === "evict-newest"
  );
}

interface CaseTabsBehaviorContextValue {
  mode: CaseTabsBehaviorMode;
  setMode: (next: CaseTabsBehaviorMode) => void;
  options: typeof CASE_TABS_BEHAVIOR_OPTIONS;
}

// Default value (not `null`) matches the DEFAULT_MODE below — a component
// that reads this outside a `CaseTabsBehaviorProvider` (an isolated test
// render, e.g.) sees exactly what a fresh, provider-wrapped session would
// see by default: tabs off. Mirrors `CaseTabsContext`'s own no-op-default
// pattern, for the same reason (many existing tests render `CsmCaseDetailPage`
// et al. standalone).
const DEFAULT_CONTEXT_VALUE: CaseTabsBehaviorContextValue = {
  mode: DEFAULT_MODE,
  setMode: () => {},
  options: CASE_TABS_BEHAVIOR_OPTIONS,
};

const CaseTabsBehaviorContext =
  createContext<CaseTabsBehaviorContextValue>(DEFAULT_CONTEXT_VALUE);

function readInitial(): CaseTabsBehaviorMode {
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    if (isBehaviorMode(saved)) return saved;
  } catch {
    /* localStorage may be unavailable — fall back to the default */
  }
  return DEFAULT_MODE;
}

/**
 * Owns the case-tabs behavior preference — same persistence shape as
 * `ThemePreferenceProvider` (localStorage only, no backend sync, read once
 * on mount). Defaults to `off`: a fresh browser/session sees the pre-feature
 * full-page-navigation behavior until the user opts in via the consolidated
 * preferences control (`PreferencesMenu`).
 */
export function CaseTabsBehaviorProvider({ children }: { children: ReactNode }): JSX.Element {
  const [mode, setModeState] = useState<CaseTabsBehaviorMode>(() => readInitial());

  const setMode = useCallback((next: CaseTabsBehaviorMode): void => {
    setModeState(next);
    try {
      window.localStorage.setItem(STORAGE_KEY, next);
    } catch {
      /* ignore — the in-memory choice still applies for this session */
    }
  }, []);

  const value = useMemo<CaseTabsBehaviorContextValue>(
    () => ({ mode, setMode, options: CASE_TABS_BEHAVIOR_OPTIONS }),
    [mode, setMode],
  );

  return (
    <CaseTabsBehaviorContext.Provider value={value}>{children}</CaseTabsBehaviorContext.Provider>
  );
}

export function useCaseTabsBehavior(): CaseTabsBehaviorContextValue {
  return useContext(CaseTabsBehaviorContext);
}
