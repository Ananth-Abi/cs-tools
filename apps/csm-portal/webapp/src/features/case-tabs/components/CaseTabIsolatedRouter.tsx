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
import { Route, Router, Routes, type To } from "react-router";
import {
  matchCaseLocation,
  pathPatternForKind,
} from "@context/case-tabs/caseRoutePatterns";
import { useCaseTabsControllerRef } from "@context/case-tabs/CaseTabsContext";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

function toHref(to: To): string {
  if (typeof to === "string") return to;
  return `${to.pathname ?? ""}${to.search ?? ""}${to.hash ?? ""}`;
}

export interface CaseTabIsolatedRouterProps {
  tab: CaseTabState;
  isVisible: boolean;
  /** The page to render for this tab — `CsmCaseDetailPage` in production,
   * swappable in tests so this component's routing/visibility mechanics can
   * be verified without pulling in the real (very large) page. */
  children: ReactNode;
}

/**
 * Mounts one case-detail page inside its own private react-router context —
 * a distinct `location` + `navigator` pair, isolated from the browser's real
 * address bar via the low-level `<Router>` primitive (the same one
 * `<BrowserRouter>`/`<MemoryRouter>` are built from).
 *
 * This is what makes "keep every open tab mounted, just hidden" possible
 * across FIVE different route bases (`/cases/:id`, `/engagements/:id`, ...):
 * `CsmCaseDetailPage` reads its `caseId` via `useParams`, which reflects
 * whichever `<Routes>` last matched — with a single real router there can
 * only ever be one such match at a time, so N simultaneously-mounted
 * instances would all resolve to the SAME (current) route's params. Each
 * instance here gets its own private match instead.
 *
 * In-page navigation (the misrouted-case redirect, the dashless-id repair in
 * `useNormalizedIdParam`, a "Related case" link, ...) is intercepted by the
 * custom navigator below rather than reaching the real browser history:
 *   - if it resolves to the SAME caseId this tab represents, the tab's own
 *     `path`/`kind` are updated in place (`updateTabPath`) — covers the
 *     redirect/repair cases, and keeps this tab's identity stable.
 *   - if it resolves to a DIFFERENT caseId (e.g. following a related-case
 *     link), it is treated as opening a new tab (or activating an existing
 *     one for that case) rather than retargeting this one — matching how the
 *     case-list entry point (`CasesList`) opens tabs, and avoiding the need
 *     to ever change a tab's React key mid-life (which would force a real
 *     remount and defeat the point of this component).
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
  const [location, setLocation] = useState(tab.path);
  // Tracked alongside `location` (not read from the `tab.kind` prop) so the
  // `<Route>` pattern below always matches `location` in the same render —
  // the prop only catches up once the dispatched `updateTabPath` action has
  // round-tripped through the parent, one render later.
  const [kind, setKind] = useState(tab.kind);
  const controllerRef = useCaseTabsControllerRef();
  // `tab.id` and `tab.caseId` are both invariant for the lifetime of a given
  // tab instance: this component is keyed by `tab.id` (never changes by
  // definition), and an in-page navigation to a DIFFERENT case is handled by
  // opening/activating a different tab (see `applyNavigation` below) rather
  // than ever retargeting this one — so `tab.caseId` never changes here
  // either. Safe to close over both directly in the memo below (with the
  // lint suppression that implies) instead of keeping them in refs.
  const navigator = useMemo(() => {
    const applyNavigation = (to: To): void => {
      const href = toHref(to);
      const pathname = href.split(/[?#]/)[0];
      const match = matchCaseLocation(pathname);
      if (!match || match.caseId === tab.caseId) {
        setLocation(href);
        if (match) {
          setKind(match.kind);
          controllerRef.current.updateTabPath(tab.id, match.kind, href);
        }
        return;
      }
      // Different case referenced from inside this tab: open/activate it as
      // its own tab, leave this tab exactly where it was.
      controllerRef.current.openTab(match.caseId, match.kind, href);
    };
    return {
      createHref: (to: To) => toHref(to),
      go: () => {
        /* No independent back/forward stack per in-app tab; not meaningful
         * here (nothing in CsmCaseDetailPage calls history.go/back today). */
      },
      push: applyNavigation,
      replace: applyNavigation,
    };
    // Stable for the lifetime of this tab instance — reads the latest
    // controller via `controllerRef` rather than depending on it directly.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div
      hidden={!isVisible}
      data-testid={`case-tab-panel-${tab.id}`}
      style={{ display: isVisible ? "flex" : "none", flexDirection: "column", flex: 1, minHeight: 0 }}
    >
      <Router location={location} navigator={navigator}>
        <Routes>
          <Route path={pathPatternForKind(kind)} element={children} />
        </Routes>
      </Router>
    </div>
  );
}
