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

import { Box, Typography } from "@wso2/oxygen-ui";
import { type JSX, Suspense, useMemo } from "react";
import { Outlet } from "react-router";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";
import SectionTabs from "@components/section-tabs/SectionTabs";
import { useRouteTabs } from "@hooks/useSectionTabs";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { hasDashboardBuilderAccess } from "@features/csm-admin/dashboards/utils/dashboardBuilderAccess";

/** The nav-node id of the "User management" tab — the only one with its own nested strip. */
const USER_MANAGEMENT_ID = "admin.user-management";

/**
 * Settings shell. Two tab levels, both driven by the navigation tree: a
 * top-level strip for [User management, Dashboards], and — only while User
 * management is the active top-level tab — a second, visually subordinate
 * strip underneath it for its own Users / Roles / Groups / Teams / Permissions
 * tabs. Which of those a deployment offers (and which are chipped as work in
 * progress) is decided by `CSM_PORTAL_FEATURE_OVERRIDES` rather than hardcoded
 * here.
 *
 * The second strip is not a bespoke nesting mechanism: `useRouteTabs` already
 * resolves a section id's children generically, so getting User management's
 * own tabs is just calling it again with `admin.user-management` instead of
 * `admin`.
 *
 * One exception: the "Dashboards" tab is additionally filtered by the
 * signed-in user's own admin role (frontend-only — see
 * `dashboardBuilderAccess.ts` for why this tab specifically needs it, unlike
 * every sibling tab here). This never removes a tab `CSM_PORTAL_FEATURE_OVERRIDES`
 * itself hid/marked WIP — it only ever narrows what a non-admin sees further.
 */
export default function CsmAdminLayout(): JSX.Element {
  const { user } = useCurrentUser();
  const isAdmin = hasDashboardBuilderAccess(user?.roles);
  const allTabs = useRouteTabs("admin");
  const tabs = useMemo(() => {
    const visible = allTabs.tabs.filter((tab) => tab.node.id !== "admin.dashboards" || isAdmin);
    // `allTabs.activeKey` was resolved against the UNFILTERED list — if
    // filtering it out here just removed the active one (a non-admin whose
    // URL still names it), fall back to this narrower list's own first tab
    // rather than handing `<Tabs>` a `value` with no matching `<Tab>`.
    const activeKey = visible.some((tab) => tab.key === allTabs.activeKey)
      ? allTabs.activeKey
      : (visible[0]?.key ?? "");
    return { ...allTabs, tabs: visible, activeKey };
  }, [allTabs, isAdmin]);

  // Always resolved (rules of hooks) — only rendered once User management is
  // the active top-level tab. Cheap: it's the same route/location read the
  // top-level hook call already does, just matched against a different node's
  // children.
  const userManagementTabs = useRouteTabs(USER_MANAGEMENT_ID);
  const activeTopNode = tabs.tabs.find((tab) => tab.key === tabs.activeKey)?.node;
  const showUserManagementTabs = activeTopNode?.id === USER_MANAGEMENT_ID;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Typography variant="h5">Settings</Typography>

      <Box sx={{ display: "flex", flexDirection: "column" }}>
        <SectionTabs {...tabs} ariaLabel="Settings tabs" scrollable />
        {showUserManagementTabs && (
          <SectionTabs
            {...userManagementTabs}
            ariaLabel="User management tabs"
            variant="secondary"
          />
        )}
      </Box>

      <Suspense fallback={<RouteSuspenseFallback />}>
        <Outlet />
      </Suspense>
    </Box>
  );
}
