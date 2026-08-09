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
  Typography,
  TextField,
  InputAdornment,
  Button,
} from "@wso2/oxygen-ui";
import { Calendar, X } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { TimeCardsDateFilterProps } from "@features/project-details/types/projectDetailsComponents";

/**
 * TimeCardsDateFilter provides a compact time-range filter for time cards.
 *
 * @param {TimeCardsDateFilterProps} props - Date values and change handlers.
 * @returns {JSX.Element} The rendered filter row.
 */
export default function TimeCardsDateFilter({
  startDate,
  endDate,
  onStartDateChange,
  onEndDateChange,
  onClear,
}: TimeCardsDateFilterProps): JSX.Element {
  const hasFilters = Boolean(startDate || endDate);

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 1.5,
        flexWrap: "wrap",
      }}
    >
      <Calendar size={18} />
      <Typography variant="body2" sx={{ fontWeight: 600 }}>
        Time Range:
      </Typography>
      <TextField
        id="time-cards-start-date"
        type="date"
        size="small"
        value={startDate}
        onChange={(e) => onStartDateChange(e.target.value)}
        sx={{ minWidth: 160 }}
        InputProps={{
          startAdornment: (
            <InputAdornment position="start">
              <Calendar size={16} />
            </InputAdornment>
          ),
        }}
      />
      <Typography variant="body2" color="text.secondary">
        to
      </Typography>
      <TextField
        id="time-cards-end-date"
        type="date"
        size="small"
        value={endDate}
        onChange={(e) => onEndDateChange(e.target.value)}
        sx={{ minWidth: 160 }}
        InputProps={{
          startAdornment: (
            <InputAdornment position="start">
              <Calendar size={16} />
            </InputAdornment>
          ),
        }}
      />
      {hasFilters && onClear && (
        <Button
          variant="text"
          size="small"
          onClick={onClear}
          startIcon={<X size={16} />}
          sx={{ color: "text.secondary" }}
        >
          Clear
        </Button>
      )}
    </Box>
  );
}
