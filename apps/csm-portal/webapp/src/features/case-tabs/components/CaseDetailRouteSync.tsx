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
import { useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";
import type { CaseRouteKind } from "@context/case-tabs/caseTabsTypes";

/**
 * The `element` for all five case-detail routes in `App.tsx` (`/cases/:id`,
 * `/engagements/:id`, `/operations/service-requests/:id`,
 * `/announcements/:id`, `/security-center/security-reports/:id`), replacing
 * a direct `<CsmCaseDetailPage/>` mount.
 *
 * Its job is narrow: given the REAL matched route (real `useParams`/
 * `useLocation` — this component is not itself isolated), ask
 * `CaseTabsContext` to open/activate an in-app tab for this case. Actually
 * rendering the page happens elsewhere, in `CaseTabsWorkspace`'s keep-alive
 * host (each open tab gets its own isolated router there — see
 * `CaseTabIsolatedRouter`), so on success this renders nothing, leaving the
 * routed `<Outlet/>` slot empty while the workspace's own content occupies
 * the same visual area.
 *
 * The one exception is the tab cap (`MAX_OPEN_CASE_TABS`): opening a 6th
 * distinct case while 5 are already open is blocked rather than evicting one
 * (see the tab strip's own design notes) — that case is rendered directly,
 * un-tabbed, exactly as this route worked before this feature existed. It
 * won't survive being navigated away from and back to, but nothing is lost
 * that wasn't already lost by a plain page navigation today.
 */
export default function CaseDetailRouteSync({
  kind,
}: {
  kind: CaseRouteKind;
}): JSX.Element | null {
  const caseId = useNormalizedIdParam("caseId");
  const location = useLocation();
  const { tabs, isAtCapacity, openTab } = useCaseTabsController();
  const { showError } = useErrorBanner();
  const warnedForCaseIdRef = useRef<string | undefined>(undefined);

  // Derived purely from already-available context state, not local state set
  // from inside the effect below — this is what this case would render if
  // `openTab` is refused, computed BEFORE the effect (which only performs
  // the actual side effect: opening the tab / firing the capacity warning)
  // ever runs. Without this, the first render for a brand new case would
  // transiently render this fallback (nothing is open for it yet) before
  // the effect's `openTab` call resolves, then immediately swap to `null`
  // once it succeeds — a needless mount-then-discard on every case opened.
  const alreadyOpenAsTab = !!caseId && tabs.some((t) => t.caseId === caseId);
  const blocked = !!caseId && !alreadyOpenAsTab && isAtCapacity;

  useEffect(() => {
    if (!caseId) return;
    const path = `${location.pathname}${location.search}${location.hash}`;
    const opened = openTab(caseId, kind, path);
    if (!opened && warnedForCaseIdRef.current !== caseId) {
      warnedForCaseIdRef.current = caseId;
      showError(
        "5 case tabs are already open — close one to open this case in a tab. " +
          "Showing it without a tab for now.",
      );
    }
  }, [caseId, kind, location.pathname, location.search, location.hash, openTab, showError]);

  if (!caseId) return null;
  if (!blocked) return null;
  return (
    <Suspense fallback={<RouteSuspenseFallback />}>
      <LazyCsmCaseDetailPage />
    </Suspense>
  );
}
