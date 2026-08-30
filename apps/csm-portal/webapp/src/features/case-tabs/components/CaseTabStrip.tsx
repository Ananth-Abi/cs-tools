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

import { Box, Chip, Tooltip } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

export interface CaseTabStripProps {
  tabs: CaseTabState[];
  activeTabId: string | null;
  onActivate: (id: string) => void;
  onRequestClose: (id: string) => void;
}

function tabDisplayLabel(tab: CaseTabState): string {
  return tab.label ?? tab.caseId;
}

/**
 * Browser-tab-like strip for in-app open case tabs, rendered by
 * `CaseTabStripBar` above the routed page content. Presentational: all
 * open/close/activate decisions (capacity, the unsaved-draft confirm) are
 * made by the caller — this component only renders the given `tabs` and
 * reports clicks. See `useCaseTabCloseConfirm` for the close-confirm dialog
 * this strip's `onRequestClose` is typically wired to.
 */
export default function CaseTabStrip({
  tabs,
  activeTabId,
  onActivate,
  onRequestClose,
}: CaseTabStripProps): JSX.Element | null {
  if (tabs.length === 0) return null;

  return (
    <Box
      role="tablist"
      aria-label="Open cases"
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 0.75,
        overflowX: "auto",
        px: 3,
        py: 1,
        borderBottom: 1,
        borderColor: "divider",
        flexShrink: 0,
        "&::-webkit-scrollbar": { height: 6 },
        "&::-webkit-scrollbar-thumb": { bgcolor: "action.disabled", borderRadius: 3 },
      }}
    >
      {tabs.map((tab) => {
        const active = tab.id === activeTabId;
        const label = tabDisplayLabel(tab);
        return (
          <Tooltip key={tab.id} title={label}>
            <Chip
              size="small"
              role="tab"
              aria-selected={active}
              label={label}
              variant={active ? "filled" : "outlined"}
              onClick={() => onActivate(tab.id)}
              onDelete={(e) => {
                // Chip's onDelete already receives a synthetic event whose
                // propagation stopping is handled by oxygen-ui internally;
                // stopPropagation here too so a delete click never also
                // triggers the Chip's own onClick (which would activate the
                // tab that's about to close).
                e.stopPropagation();
                onRequestClose(tab.id);
              }}
              aria-label={`Close ${label}`}
              sx={{
                flexShrink: 0,
                maxWidth: 220,
                cursor: "pointer",
                ...(active ? { bgcolor: "action.selected", fontWeight: 600 } : {}),
              }}
            />
          </Tooltip>
        );
      })}
    </Box>
  );
}
