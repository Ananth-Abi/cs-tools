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

import { Box, Chip, Tab, Tabs } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { Navigate } from "react-router";
import type { SectionTabsState } from "@hooks/useSectionTabs";
import { firstEnabledTabHref } from "@hooks/useSectionTabs";

interface SectionTabsProps extends SectionTabsState {
  /** Accessible name for the strip, e.g. "Operations tabs". */
  ariaLabel: string;
  scrollable?: boolean;
  /**
   * Visual weight. `"primary"` (default) is a section's own tab strip.
   * `"secondary"` renders smaller and indented, for a strip that belongs to
   * one of those tabs rather than to the section itself — e.g. Settings'
   * "User management" tab has its own row of sub-tabs underneath the primary
   * strip.
   */
  variant?: "primary" | "secondary";
}

/**
 * A section's tab strip, rendered from the navigation tree. Also doubles as a
 * nested tab's own strip via `variant="secondary"` — the underlying data
 * (`useRouteTabs`/`useQueryTabs`) is already resolved per nav-node id, so a
 * second level is just a second `<SectionTabs>` fed by a second hook call,
 * not a different component.
 *
 * A tab the deployment marked WIP stays in the strip but is disabled and
 * chipped, so the section still advertises what is coming without offering a
 * dead panel; a hidden tab never reaches this component. Renders nothing when
 * the section has no visible tabs at all, which only happens if every tab is
 * hidden by config.
 */
export default function SectionTabs({
  tabs,
  activeKey,
  select,
  ariaLabel,
  scrollable = false,
  variant = "primary",
}: SectionTabsProps): JSX.Element | null {
  if (tabs.length === 0) return null;
  const isSecondary = variant === "secondary";

  return (
    <Box
      sx={{
        borderBottom: 1,
        borderColor: "divider",
        ...(isSecondary && { pl: 2 }),
      }}
    >
      <Tabs
        aria-label={ariaLabel}
        value={activeKey}
        onChange={(_, key: string) => select(key)}
        variant={scrollable ? "scrollable" : "standard"}
        scrollButtons={scrollable ? "auto" : false}
        sx={
          isSecondary
            ? {
                minHeight: 36,
                "& .MuiTab-root": {
                  minHeight: 36,
                  paddingTop: 0.5,
                  paddingBottom: 0.5,
                  fontSize: "0.8125rem",
                },
              }
            : undefined
        }
      >
        {tabs.map((tab) =>
          tab.state === "wip" ? (
            // The chip carries the "not ready yet" message on its own: a
            // Tooltip would never fire here, because a disabled MUI Tab drops
            // pointer events for its whole subtree.
            <Tab
              key={tab.key}
              value={tab.key}
              disabled
              label={
                <Box
                  component="span"
                  sx={{ display: "flex", alignItems: "center", gap: 1 }}
                >
                  {tab.label}
                  <Chip
                    size="small"
                    label="WIP"
                    color="warning"
                    variant="outlined"
                    sx={{ height: 18, fontSize: 10 }}
                  />
                </Box>
              }
            />
          ) : (
            <Tab key={tab.key} value={tab.key} label={tab.label} />
          ),
        )}
      </Tabs>
    </Box>
  );
}

/**
 * Index-route element for a tabbed section: sends the user to its first usable
 * tab rather than to a hardcoded one that this deployment may have restricted.
 * Falls back to the dashboard when every tab is hidden.
 */
export function SectionIndexRedirect({
  sectionId,
}: {
  sectionId: string;
}): JSX.Element {
  return <Navigate to={firstEnabledTabHref(sectionId) ?? "/dashboard"} replace />;
}
