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
import LazyCsmCaseDetailPage from "@features/case-tabs/lazyCaseDetailPage";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";
import { basePathForKind, matchCaseLocation } from "@context/case-tabs/caseRoutePatterns";
import { useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import { useNavTransition } from "@hooks/useNavTransition";
import CaseTabIsolatedRouter from "@features/case-tabs/components/CaseTabIsolatedRouter";
import CaseTabLabel from "@features/case-tabs/components/CaseTabLabel";
import CaseTabStrip from "@features/case-tabs/components/CaseTabStrip";
import { useCaseTabCloseConfirm } from "@features/case-tabs/hooks/useCaseTabCloseConfirm";

/**
 * The visible tab strip, meant to sit ABOVE the routed page content (e.g.
 * directly above `AppLayout`'s scrollable content region) — full-bleed,
 * outside that region's own padding, so it reads as a persistent strip like
 * a browser's, not part of the page underneath it.
 *
 * Also mounts each open tab's `CaseTabLabel` (label resolution has no visual
 * output of its own — see that component) and the close-confirm dialog.
 * Renders nothing when no tabs are open.
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
      {tabs.map((tab) => (
        <CaseTabLabel key={tab.id} tab={tab} />
      ))}
    </>
  );
}

/**
 * The keep-alive host: every open tab's `CsmCaseDetailPage`, each in its own
 * isolated router (`CaseTabIsolatedRouter`), always mounted and hidden via
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
 * none are left open.
 */
export function CaseTabsContentHost(): JSX.Element | null {
  const location = useLocation();
  const navigate = useNavTransition();
  const { tabs, activeTabId } = useCaseTabsController();

  const currentMatch = matchCaseLocation(location.pathname);
  const isCaseRouteActive = currentMatch !== undefined;

  const prevTabIdsRef = useRef<Set<string>>(new Set(tabs.map((t) => t.id)));
  useEffect(() => {
    const prevIds = prevTabIdsRef.current;
    prevTabIdsRef.current = new Set(tabs.map((t) => t.id));
    if (!currentMatch) return;
    const stillOpenForCurrentRoute = tabs.some((t) => t.caseId === currentMatch.caseId);
    if (stillOpenForCurrentRoute) return;
    const hadAnyTabBefore = prevIds.size > 0;
    if (!hadAnyTabBefore) return;
    const nextActive = tabs.find((t) => t.id === activeTabId);
    navigate(nextActive ? nextActive.path : basePathForKind(currentMatch.kind), {
      replace: true,
    });
    // Only re-run when the open tab set or the route changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tabs, currentMatch?.caseId]);

  if (tabs.length === 0) return null;

  // `display: none` (not just hiding each panel) when no case route is
  // current, so this host takes no part in the parent flex layout at all —
  // it must not compete for space with whatever page IS showing (e.g. the
  // dashboard) while tabs sit dormant in the background.
  return (
    <div
      style={{
        display: isCaseRouteActive ? "flex" : "none",
        flexDirection: "column",
        flex: 1,
        minHeight: 0,
      }}
    >
      <Suspense fallback={<RouteSuspenseFallback />}>
        {tabs.map((tab) => (
          <CaseTabIsolatedRouter
            key={tab.id}
            tab={tab}
            isVisible={isCaseRouteActive && tab.id === activeTabId}
          >
            <LazyCsmCaseDetailPage />
          </CaseTabIsolatedRouter>
        ))}
      </Suspense>
    </div>
  );
}
