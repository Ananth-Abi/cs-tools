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

import { Suspense, useEffect, useRef, type JSX } from "react";
import { useLocation } from "react-router";
import { pageComponentForKind } from "@features/case-tabs/tabPageRegistry";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";
import { basePathForKind, matchCaseLocation } from "@context/case-tabs/caseRoutePatterns";
import { useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import { useNavTransition } from "@hooks/useNavTransition";
import CaseTabIsolatedRouter from "@features/case-tabs/components/CaseTabIsolatedRouter";
import CaseTabStrip from "@features/case-tabs/components/CaseTabStrip";
import { useCaseTabCloseConfirm } from "@features/case-tabs/hooks/useCaseTabCloseConfirm";

/**
 * The visible tab strip, meant to sit ABOVE the routed page content (e.g.
 * directly above `AppLayout`'s scrollable content region) — full-bleed,
 * outside that region's own padding, so it reads as a persistent strip like
 * a browser's, not part of the page underneath it.
 *
 * Also mounts the close-confirm dialog. Each tab's own label is reported
 * directly by the page rendering it (`useReportCaseTabMeta`, called from
 * `CsmCaseDetailPage`) rather than fetched separately here — see that
 * hook's own doc comment for why. Renders nothing when no tabs are open.
 */
export function CaseTabStripBar(): JSX.Element | null {
  const location = useLocation();
  const navigate = useNavTransition();
  const { tabs, activeTabId, setActiveTab } = useCaseTabsController();
  const { requestClose, dialog } = useCaseTabCloseConfirm();

  if (tabs.length === 0) return null;

  return (
    <>
      <CaseTabStrip
        tabs={tabs}
        activeTabId={activeTabId}
        onActivate={(id) => {
          const tab = tabs.find((t) => t.id === id);
          if (!tab) return;
          setActiveTab(id);
          if (location.pathname !== tab.path.split(/[?#]/)[0]) {
            navigate(tab.path);
          }
        }}
        onRequestClose={(id) => {
          const tab = tabs.find((t) => t.id === id);
          if (tab) requestClose(tab);
        }}
      />
      {dialog}
    </>
  );
}

/**
 * The keep-alive host: every open tab's page (whichever one
 * `pageComponentForKind` says its kind renders — `CsmCaseDetailPage` for the
 * five case-like kinds, `CsmIncidentDetailPage`/`CsmChangeRequestDetailPage`
 * for those two), each in its own isolated identity (`CaseTabIsolatedRouter`
 * / `CaseRouteOverrideContext`), always mounted and hidden via
 * CSS unless it is both the active tab AND the current real route is a
 * case-detail route (`matchCaseLocation`) — so navigating to an unrelated
 * page (e.g. the dashboard) hides every tab's content without unmounting any
 * of them, and navigating back shows the right one again with all its state
 * intact.
 *
 * Meant to render in the exact spot the routed `<Outlet/>` normally occupies
 * (`AppLayout` renders this as a sibling immediately before its own
 * `{children || <Outlet/>}`), inheriting that region's padding/scroll
 * styling — see `AppLayout`'s own comment at that call site. Renders `null`
 * (not even a wrapper) when no tabs are open, so `<Outlet/>` is the only
 * thing occupying that space for every page that never touches a case
 * route.
 *
 * Also owns closing a tab whose current route is the one just closed:
 * navigates to whatever tab became active, or that case type's list view if
 * none are left open. Deliberately does NOT do this for a case that was
 * simply never opened as a tab at all (the open-tab-cap fallback in
 * `CaseDetailRouteSync`, rendered un-tabbed via the real `<Outlet/>`) — an
 * earlier version of this effect couldn't tell those two situations apart
 * (both look like "no tab backs the current route") and silently redirected
 * a just-clicked, cap-blocked case's URL back to whatever tab happened to be
 * active, which looked like the click had done nothing. Distinguishing them
 * needs the PREVIOUS render's open caseIds, not just tab ids: a genuinely
 * closed tab's caseId was in that set; a never-opened (blocked) one never
 * was.
 */
export function CaseTabsContentHost(): JSX.Element | null {
  const location = useLocation();
  const navigate = useNavTransition();
  const { tabs, activeTabId } = useCaseTabsController();

  const currentMatch = matchCaseLocation(location.pathname);
  // Whether a tab actually backs the CURRENT route's specific case — not
  // merely "the current path looks like a case-detail route", which is also
  // true for a case rendered un-tabbed past the open-tab cap. Gating this
  // host's visibility on the latter (as an earlier version did) meant it
  // stayed visible — showing whichever tab was last active — right on top
  // of that un-tabbed page's own real content.
  const activeRouteHasTab =
    !!currentMatch && tabs.some((t) => t.caseId === currentMatch.caseId);

  const prevCaseIdsRef = useRef<Set<string>>(new Set(tabs.map((t) => t.caseId)));
  useEffect(() => {
    const prevCaseIds = prevCaseIdsRef.current;
    prevCaseIdsRef.current = new Set(tabs.map((t) => t.caseId));
    if (!currentMatch || activeRouteHasTab) return;
    // Only redirect when a tab that WAS backing this exact case just closed
    // — never for a case that was blocked by the open-tab cap (never
    // tab-managed to begin with) or one visited for the very first time,
    // both of which also fail `activeRouteHasTab` above but must be left
    // alone to render their own (un-tabbed) content undisturbed.
    const wasOpenForThisCase = prevCaseIds.has(currentMatch.caseId);
    if (!wasOpenForThisCase) return;
    const nextActive = tabs.find((t) => t.id === activeTabId);
    navigate(nextActive ? nextActive.path : basePathForKind(currentMatch.kind), {
      replace: true,
    });
    // Only re-run when the open tab set or the route changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tabs, currentMatch?.caseId, activeRouteHasTab]);

  if (tabs.length === 0) return null;

  // `display: none` (not just hiding each panel) unless a tab actually backs
  // the current route, so this host takes no part in the parent flex layout
  // at all in that case — it must not compete for space with (or visually
  // sit on top of) whatever page IS showing instead: the dashboard while
  // tabs sit dormant in the background, or a cap-blocked case's own
  // un-tabbed real content.
  return (
    <div
      style={{
        display: activeRouteHasTab ? "flex" : "none",
        flexDirection: "column",
        flex: 1,
        minHeight: 0,
      }}
    >
      <Suspense fallback={<RouteSuspenseFallback />}>
        {tabs.map((tab) => {
          const Page = pageComponentForKind(tab.kind);
          return (
            <CaseTabIsolatedRouter
              key={tab.id}
              tab={tab}
              isVisible={activeRouteHasTab && tab.id === activeTabId}
            >
              <Page />
            </CaseTabIsolatedRouter>
          );
        })}
      </Suspense>
    </div>
  );
}
