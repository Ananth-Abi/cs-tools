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

import { useMemo, useState, type JSX, type ReactNode } from "react";
import type { NavigateOptions, To } from "react-router";
import { matchCaseLocation } from "@context/case-tabs/caseRoutePatterns";
import { useCaseTabsControllerRef } from "@context/case-tabs/CaseTabsContext";
import { CaseRouteOverrideProvider } from "@context/case-tabs/CaseRouteOverrideContext";
import type { CaseRouteKind, CaseTabState } from "@context/case-tabs/caseTabsTypes";

function toHref(to: To): string {
  if (typeof to === "string") return to;
  return `${to.pathname ?? ""}${to.search ?? ""}${to.hash ?? ""}`;
}

export interface CaseTabIsolatedRouterProps {
  tab: CaseTabState;
  isVisible: boolean;
  /** The page to render for this tab — `CsmCaseDetailPage` in production,
   * swappable in tests so this component's mechanics can be verified without
   * pulling in the real (very large) page. */
  children: ReactNode;
}

/**
 * Mounts one case-detail page "kept alive" in the background, giving it its
 * own private `caseId`/location/navigate — WITHOUT a second react-router
 * `<Router>`. react-router refuses to render a `<Router>` inside another
 * `<Router>` (an unconditional invariant), and the app already has exactly
 * one (`<BrowserRouter>`, in `App.tsx`) — an earlier version of this
 * component tried exactly that (a low-level `<Router>` per tab) and crashed
 * the moment any case was opened for it. See `CaseRouteOverrideContext`'s
 * own doc comment for the full explanation of why a plain Context works
 * where a second Router cannot: `CsmCaseDetailPage` still reads the REAL
 * `useParams`/`useLocation`/`useNavigate` (there is only ever one, real,
 * app-wide match), it just prefers this override's values when one is
 * present in context.
 *
 * In-page navigation (the misrouted-case redirect, the dashless-id repair in
 * `useNormalizedIdParam`, ...) is intercepted by the `navigate` function
 * passed through the override rather than reaching the real browser history:
 *   - if it resolves to the SAME caseId this tab represents, the tab's own
 *     `path`/`kind` are updated in place (`updateTabPath`) — covers the
 *     redirect/repair cases, and keeps this tab's identity stable.
 *   - if it resolves to a DIFFERENT caseId (e.g. following a related-case
 *     link — though in practice those render as real react-router `<Link>`s
 *     bound to the real router regardless of which tab they're clicked from,
 *     so they already open/activate a tab via `CaseDetailRouteSync` before
 *     ever reaching this code path), it is treated as opening a new tab (or
 *     activating an existing one for that case) rather than retargeting this
 *     one — avoiding the need to ever change a tab's React key mid-life
 *     (which would force a real remount and defeat the point of this
 *     component).
 *
 * The active tab's real-URL sync (so a reload/bookmark on `/cases/:id`
 * still works — see this feature's design notes) is owned by the caller
 * (`CaseTabsWorkspace`), not here: this component never touches
 * `window.location` itself.
 */
export default function CaseTabIsolatedRouter({
  tab,
  isVisible,
  children,
}: CaseTabIsolatedRouterProps): JSX.Element {
  const initialPath = useMemo(() => {
    const [pathnameAndSearch, hash = ""] = tab.path.split("#");
    const [pathname, search = ""] = pathnameAndSearch.split("?");
    return {
      pathname,
      search: search ? `?${search}` : "",
      hash: hash ? `#${hash}` : "",
    };
  }, [tab.path]);

  const [routeState, setRouteState] = useState<{
    pathname: string;
    search: string;
    hash: string;
    kind: CaseRouteKind;
    state: unknown;
  }>({ ...initialPath, kind: tab.kind, state: tab.state });

  const controllerRef = useCaseTabsControllerRef();

  // `tab.id` and `tab.caseId` are both invariant for the lifetime of a given
  // tab instance: this component is keyed by `tab.id` (never changes by
  // definition), and an in-page navigation to a DIFFERENT case is handled by
  // opening/activating a different tab (see `navigate` below) rather than
  // ever retargeting this one — so `tab.caseId` never changes here either.
  // Safe to close over both directly in the memo below (with the lint
  // suppression that implies) instead of keeping them in refs.
  const navigate = useMemo(() => {
    return (to: To | number, options?: NavigateOptions): void => {
      if (typeof to === "number") {
        // No independent back/forward stack per in-app tab; not meaningful
        // here (nothing in CsmCaseDetailPage calls navigate(-1)/(1) today).
        return;
      }
      const href = toHref(to);
      const pathname = href.split(/[?#]/)[0];
      const search = href.includes("?") ? `?${href.split("?")[1]?.split("#")[0]}` : "";
      const hash = href.includes("#") ? `#${href.split("#")[1]}` : "";
      const match = matchCaseLocation(pathname);
      if (!match || match.caseId === tab.caseId) {
        setRouteState((prev) => ({
          pathname,
          search,
          hash,
          kind: match?.kind ?? prev.kind,
          state: options?.state,
        }));
        if (match) {
          controllerRef.current.updateTabPath(tab.id, match.kind, href);
        }
        return;
      }
      // Different case referenced from inside this tab: open/activate it as
      // its own tab, leave this tab exactly where it was.
      controllerRef.current.openTab(match.caseId, match.kind, href, options?.state);
    };
    // Stable for the lifetime of this tab instance — reads the latest
    // controller via `controllerRef` rather than depending on it directly.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const overrideValue = useMemo(
    () => ({
      caseId: tab.caseId,
      kind: routeState.kind,
      pathname: routeState.pathname,
      search: routeState.search,
      hash: routeState.hash,
      state: routeState.state,
      navigate,
    }),
    [tab.caseId, routeState, navigate],
  );

  return (
    <div
      hidden={!isVisible}
      data-testid={`case-tab-panel-${tab.id}`}
      style={{
        display: isVisible ? "flex" : "none",
        flexDirection: "column",
        flex: 1,
        minHeight: 0,
      }}
    >
      <CaseRouteOverrideProvider value={overrideValue}>{children}</CaseRouteOverrideProvider>
    </div>
  );
}
