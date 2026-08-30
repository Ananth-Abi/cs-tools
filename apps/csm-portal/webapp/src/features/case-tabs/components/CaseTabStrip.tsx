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

import { Box, Chip, Menu, MenuItem, Tooltip } from "@wso2/oxygen-ui";
import { useState, type JSX, type MouseEvent as ReactMouseEvent } from "react";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

export interface PinnedTabProps {
  label: string;
  active: boolean;
  onClick: () => void;
}

export interface CaseTabStripProps {
  tabs: CaseTabState[];
  activeTabId: string | null;
  onActivate: (id: string) => void;
  onRequestClose: (id: string) => void;
  /** Right-click "Close all tabs" (on a chip or on empty strip space). */
  onCloseAll: () => void;
  /** Right-click "Close other tabs" — every open tab except `keepId`. */
  onCloseOthers: (keepId: string) => void;
  /** The permanent, non-closable "wherever the user currently is" tab at
   * position 0 — see `useCurrentLocationTab`. Optional purely so this
   * component's own tests can exercise the plain case-tab strip in
   * isolation; `CaseTabStripBar` always supplies one. Never rendered when
   * `tabs` is empty — the whole strip hides in that case (see this
   * component's own doc comment). */
  pinnedTab?: PinnedTabProps;
}

// Shown while a tab's own page hasn't reported a label yet (see
// `useReportCaseTabMeta`) — deliberately not the raw caseId/UUID, which read
// as a rendering glitch rather than "still loading".
const LOADING_LABEL = "Loading…";

function tabDisplayLabel(tab: CaseTabState): string {
  return tab.label ?? LOADING_LABEL;
}

/** Tooltip content: internal/project-scoped id + subject — a fuller
 * identity than the chip's own short number-only label. Falls back to the
 * chip label alone while the record's own data (and so its tooltip fields)
 * hasn't resolved yet. */
function tabTooltip(tab: CaseTabState): string {
  if (!tab.internalId && !tab.subject) return tabDisplayLabel(tab);
  return [tab.internalId, tab.subject].filter(Boolean).join(" · ");
}

type ContextMenuTarget = { kind: "tab"; tabId: string } | { kind: "empty" };

type MenuAnchorPosition = { top: number; left: number };

/**
 * Browser-tab-like strip for in-app open tabs, rendered by `CaseTabStripBar`
 * above the routed page content. Presentational: all open/close/activate
 * decisions (capacity, the unsaved-draft confirm) are made by the caller —
 * this component only renders the given `tabs` (plus the pinned tab, if
 * given) and reports clicks. See `useCaseTabCloseConfirm` for the
 * close-confirm dialog this strip's `onRequestClose` is typically wired to.
 *
 * Renders nothing at all when `tabs` is empty — including the pinned tab,
 * which would otherwise sit alone taking up a full strip's worth of space
 * for no case tabs open. The pinned tab only appears once the first case tab
 * does.
 *
 * Right-clicking a tab chip (or empty space in the strip, when there's
 * nowhere else to click) opens a small context menu — "Close all tabs" /
 * "Close other tabs" — same `onContextMenu` + anchored `<Menu>` pattern as
 * `PinnedTabs`' own right-click rename menu elsewhere in this codebase. The
 * pinned tab is never part of either action (it isn't closable at all).
 */
export default function CaseTabStrip({
  tabs,
  activeTabId,
  onActivate,
  onRequestClose,
  onCloseAll,
  onCloseOthers,
  pinnedTab,
}: CaseTabStripProps): JSX.Element | null {
  const [menuAnchorPosition, setMenuAnchorPosition] = useState<MenuAnchorPosition | null>(null);
  const [menuTarget, setMenuTarget] = useState<ContextMenuTarget | null>(null);

  if (tabs.length === 0) return null;

  const closeContextMenu = (): void => {
    setMenuAnchorPosition(null);
    setMenuTarget(null);
  };

  const openContextMenu = (e: ReactMouseEvent<HTMLElement>, target: ContextMenuTarget): void => {
    e.preventDefault();
    // Anchor at the cursor, not the triggering element — an
    // element-anchored `<Menu>` opens at that element's top-left corner
    // (MUI's `anchorEl` default), which for the strip's own empty-space
    // right-click means the strip's full-width container: the menu would
    // always render at the strip's left edge regardless of where within it
    // was actually clicked.
    setMenuAnchorPosition({ top: e.clientY, left: e.clientX });
    setMenuTarget(target);
  };

  return (
    <Box
      role="tablist"
      aria-label="Open cases"
      onContextMenu={(e: ReactMouseEvent<HTMLElement>) => {
        // Only when the strip's own background was right-clicked, not a
        // chip inside it — each chip has its own onContextMenu, which stops
        // this one from also firing (see its stopPropagation below).
        openContextMenu(e, { kind: "empty" });
      }}
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
      {pinnedTab && (
        <Tooltip title={pinnedTab.label}>
          {/* No `onDelete` and no `onContextMenu` — this tab is permanent,
              not part of the closable case-tab set (see
              `useCurrentLocationTab`'s doc comment), so it's excluded from
              both context-menu actions and never itself a right-click
              target. */}
          <Chip
            size="small"
            role="tab"
            aria-selected={pinnedTab.active}
            label={pinnedTab.label}
            variant={pinnedTab.active ? "filled" : "outlined"}
            onClick={pinnedTab.onClick}
            sx={{
              flexShrink: 0,
              maxWidth: 220,
              cursor: "pointer",
              fontStyle: "italic",
              ...(pinnedTab.active ? { bgcolor: "action.selected", fontWeight: 600 } : {}),
            }}
          />
        </Tooltip>
      )}
      {tabs.map((tab) => {
        const active = tab.id === activeTabId;
        const label = tabDisplayLabel(tab);
        return (
          <Tooltip key={tab.id} title={tabTooltip(tab)}>
            <Chip
              size="small"
              role="tab"
              aria-selected={active}
              label={label}
              variant={active ? "filled" : "outlined"}
              onClick={() => onActivate(tab.id)}
              onContextMenu={(e: ReactMouseEvent<HTMLElement>) => {
                e.stopPropagation();
                openContextMenu(e, { kind: "tab", tabId: tab.id });
              }}
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

      <Menu
        open={Boolean(menuAnchorPosition)}
        onClose={closeContextMenu}
        anchorReference="anchorPosition"
        anchorPosition={menuAnchorPosition ?? undefined}
      >
        {menuTarget?.kind === "tab" && (
          <MenuItem
            onClick={() => {
              if (menuTarget.kind === "tab") onCloseOthers(menuTarget.tabId);
              closeContextMenu();
            }}
          >
            Close other tabs
          </MenuItem>
        )}
        <MenuItem
          onClick={() => {
            onCloseAll();
            closeContextMenu();
          }}
        >
          Close all tabs
        </MenuItem>
      </Menu>
    </Box>
  );
}
