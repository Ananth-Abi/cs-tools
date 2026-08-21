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

import {
  Box,
  Button,
  Checkbox,
  Divider,
  IconButton,
  ListItemText,
  MenuItem,
  MenuList,
  Popover,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronDown, ChevronUp, ColumnsSettings } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type MouseEvent } from "react";
import type { ColumnOption } from "@hooks/useColumnPreferences";

export interface ColumnCustomizerButtonProps {
  /** All known columns for this table, in the user's current order —
   * `useColumnPreferences`'s `allColumns`. */
  allColumns: ColumnOption[];
  isVisible: (id: string) => boolean;
  onToggle: (id: string) => void;
  onMove: (id: string, direction: "up" | "down") => void;
  onReset: () => void;
  /** Accessible label for the trigger button; defaults to "Customise
   * columns". Override when a page has more than one table on screen. */
  label?: string;
}

/**
 * "Customise columns" trigger + popover: check/uncheck to add or remove a
 * column, up/down arrows to reorder it. Shared across every table that adopts
 * `useColumnPreferences` — the table itself owns what each column id renders
 * as; this component only edits the visibility/order state.
 */
export default function ColumnCustomizerButton({
  allColumns,
  isVisible,
  onToggle,
  onMove,
  onReset,
  label = "Customise columns",
}: ColumnCustomizerButtonProps): JSX.Element {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const open = Boolean(anchorEl);
  const visibleCount = allColumns.filter((c) => isVisible(c.id)).length;

  const handleOpen = (e: MouseEvent<HTMLElement>): void => setAnchorEl(e.currentTarget);
  const handleClose = (): void => setAnchorEl(null);

  return (
    <>
      <Tooltip title={label}>
        <IconButton size="small" aria-label={label} onClick={handleOpen}>
          <ColumnsSettings size={16} />
        </IconButton>
      </Tooltip>
      <Popover
        open={open}
        anchorEl={anchorEl}
        onClose={handleClose}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        transformOrigin={{ vertical: "top", horizontal: "right" }}
      >
        <Box sx={{ width: 280, py: 1 }}>
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{ px: 2, py: 0.5, display: "block", fontWeight: 600 }}
          >
            Columns
          </Typography>
          <MenuList dense aria-label={label} sx={{ maxHeight: 320, overflowY: "auto" }}>
            {allColumns.map((column, index) => {
              const checked = isVisible(column.id);
              const disableUncheck = checked && visibleCount <= 1;
              return (
                <MenuItem
                  key={column.id}
                  disableRipple
                  onClick={() => !disableUncheck && onToggle(column.id)}
                  sx={{ py: 0.25 }}
                >
                  <Checkbox
                    size="small"
                    checked={checked}
                    disabled={disableUncheck}
                    tabIndex={-1}
                    sx={{ mr: 1, p: 0.25 }}
                  />
                  <ListItemText
                    primary={column.label}
                    slotProps={{ primary: { style: { fontSize: 13 } } }}
                  />
                  <Box sx={{ display: "flex", ml: 1 }}>
                    <IconButton
                      size="small"
                      aria-label={`Move ${column.label} up`}
                      disabled={index === 0}
                      onClick={(e) => {
                        e.stopPropagation();
                        onMove(column.id, "up");
                      }}
                      sx={{ p: 0.25 }}
                    >
                      <ChevronUp size={14} />
                    </IconButton>
                    <IconButton
                      size="small"
                      aria-label={`Move ${column.label} down`}
                      disabled={index === allColumns.length - 1}
                      onClick={(e) => {
                        e.stopPropagation();
                        onMove(column.id, "down");
                      }}
                      sx={{ p: 0.25 }}
                    >
                      <ChevronDown size={14} />
                    </IconButton>
                  </Box>
                </MenuItem>
              );
            })}
          </MenuList>
          <Divider sx={{ my: 0.5 }} />
          <Box sx={{ px: 1.5 }}>
            <Button size="small" variant="text" onClick={onReset} sx={{ textTransform: "none" }}>
              Reset to default
            </Button>
          </Box>
        </Box>
      </Popover>
    </>
  );
}
