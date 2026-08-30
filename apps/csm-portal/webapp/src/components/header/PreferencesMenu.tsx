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
  IconButton,
  MenuItem,
  Popover,
  Select,
  Tooltip,
  Typography,
  type SelectChangeEvent,
} from "@wso2/oxygen-ui";
import { Settings } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type MouseEvent } from "react";
import { useThemePreference } from "@context/theme/ThemePreferenceContext";
import { isThemeKey } from "@config/themeConfig";
import {
  useCaseTabsBehavior,
  type CaseTabsBehaviorMode,
} from "@context/case-tabs/CaseTabsBehaviorContext";

function isBehaviorMode(value: string): value is CaseTabsBehaviorMode {
  return (
    value === "off" || value === "block" || value === "evict-oldest" || value === "evict-newest"
  );
}

/**
 * Single consolidated header entry point for user-level display/behavior
 * preferences: the Oxygen UI theme (previously its own standalone dropdown,
 * `ThemeSelect`) and the case-tabs behavior mode (`CaseTabsBehaviorContext`).
 * Both are localStorage-only, no-backend-sync preferences with the same
 * persistence shape, so one small popover holds both rather than growing the
 * header with a second standalone control per preference added.
 */
export default function PreferencesMenu(): JSX.Element {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const open = Boolean(anchorEl);

  const { themeKey, setThemeKey, options: themeOptions } = useThemePreference();
  const { mode, setMode, options: behaviorOptions } = useCaseTabsBehavior();

  const handleOpen = (e: MouseEvent<HTMLElement>): void => setAnchorEl(e.currentTarget);
  const handleClose = (): void => setAnchorEl(null);

  const handleThemeChange = (e: SelectChangeEvent<string>): void => {
    const next = e.target.value;
    if (isThemeKey(next)) setThemeKey(next);
  };

  const handleBehaviorChange = (e: SelectChangeEvent<string>): void => {
    const next = e.target.value;
    if (isBehaviorMode(next)) setMode(next);
  };

  return (
    <>
      <Tooltip title="Preferences">
        <IconButton size="small" aria-label="Preferences" onClick={handleOpen}>
          <Settings size={16} />
        </IconButton>
      </Tooltip>
      <Popover
        open={open}
        anchorEl={anchorEl}
        onClose={handleClose}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        transformOrigin={{ vertical: "top", horizontal: "right" }}
      >
        <Box sx={{ width: 300, p: 2, display: "flex", flexDirection: "column", gap: 2 }}>
          <Box>
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", fontWeight: 600, mb: 0.5 }}
            >
              Theme
            </Typography>
            <Select
              value={themeKey}
              onChange={handleThemeChange}
              size="small"
              fullWidth
              aria-label="Select theme"
            >
              {themeOptions.map((o) => (
                <MenuItem key={o.key} value={o.key}>
                  {o.label}
                </MenuItem>
              ))}
            </Select>
          </Box>
          <Box>
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", fontWeight: 600, mb: 0.5 }}
            >
              Case tabs (beta)
            </Typography>
            <Select
              value={mode}
              onChange={handleBehaviorChange}
              size="small"
              fullWidth
              aria-label="Case tabs behavior"
            >
              {behaviorOptions.map((o) => (
                <MenuItem key={o.mode} value={o.mode}>
                  {o.label}
                </MenuItem>
              ))}
            </Select>
          </Box>
        </Box>
      </Popover>
    </>
  );
}
