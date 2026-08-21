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

import { Autocomplete, Box, Checkbox, ListItemText, TextField, Tooltip } from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import type * as React from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchTags } from "@features/csm-cases/api/useSearchTags";

interface TagsMultiSelectProps {
  id?: string;
  label?: string;
  /** Selected free-text tag labels (the filter values themselves). */
  values: string[];
  onChange: (next: string[]) => void;
}

/**
 * Tag filter for the cases list, searching already-used tag labels from the
 * backend as the user types (`useSearchTags`, the same `/tags/search`
 * type-ahead {@link AddTagDialog} uses) — the tag-side twin of
 * {@link AsyncProjectMultiSelect}/{@link AsyncAssigneeMultiSelect}. Stays
 * `freeSolo`: unlike a project or an engineer, a tag is a genuinely free-text
 * label (SN's generic label mechanism, e.g. `micro-gw`, `ws-policy`) with no
 * canonical existence check — filtering by a label the search didn't
 * surface (a typo, or one used by a case outside the current query's top 20)
 * is still a meaningful, harmless filter, so typing one in and pressing
 * Enter/comma must keep working even with no matching suggestion.
 */
export default function TagsMultiSelect({
  id = "cases-filter-tags",
  label = "Tags",
  values,
  onChange,
}: TagsMultiSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  // Enabled while the dropdown is open, so it loads a first batch of
  // suggestions on open (no typing needed) and re-queries as the user types.
  const { data, isFetching, isError } = useSearchTags(query, open);

  // Pool = current selection (so the field renders its chips) + the search
  // results' labels, de-duplicated.
  const options: string[] = useMemo(() => {
    const seen = new Set(values);
    const results = (data ?? [])
      .map((t) => t.label)
      .filter((l) => l.length > 0 && !seen.has(l));
    return [...values, ...results];
  }, [data, values]);

  return (
    <Autocomplete<string, true, false, true>
      multiple
      freeSolo
      // MUI hides the dropdown chevron by default for any `freeSolo`
      // Autocomplete (`hasPopupIcon` in its source is `!freeSolo` unless
      // this is set) -- every other filter control in this bar (Select- and
      // Autocomplete-based alike) shows one, so force it back on here too.
      forcePopupIcon
      size="small"
      id={id}
      options={options}
      value={values}
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      // Stays open across a selection -- same reasoning as
      // AsyncProjectMultiSelect/AsyncAssigneeMultiSelect: this is a
      // multi-select, so picking one tag shouldn't close the dropdown on
      // someone about to pick several from the same search.
      disableCloseOnSelect
      loading={isFetching && (data ?? []).length === 0}
      // The backend already filtered by the typed term; don't re-filter
      // locally (that would also drop the currently-selected values, which
      // must stay in `options` so their chips render).
      filterOptions={(opts) => opts}
      sx={{ "& .MuiAutocomplete-inputRoot": { flexWrap: "nowrap", minHeight: 40 } }}
      onChange={(_event, next) =>
        onChange(next.map((v) => v.trim()).filter((v) => v.length > 0))
      }
      inputValue={input}
      onInputChange={(_event, value, reason) => {
        // Keep the typed term after a selection (reason "reset") so the user
        // can pick several from one search; clear only on explicit input/clear.
        if (reason === "input" || reason === "clear") setInput(value);
      }}
      noOptionsText={
        isError
          ? "Couldn't load tags. Try again."
          : isFetching
            ? "Loading tags…"
            : "No matching tags — press Enter to filter by it anyway"
      }
      renderOption={(props, option, { selected }) => {
        const { key, ...liProps } = props as React.HTMLAttributes<HTMLLIElement> & {
          key: string;
        };
        return (
          <li key={key} {...liProps} style={{ paddingTop: 2, paddingBottom: 2 }}>
            <Checkbox size="small" checked={selected} sx={{ mr: 1, p: 0.25 }} />
            <ListItemText
              primary={option}
              slotProps={{ primary: { style: { fontSize: 13 } } }}
            />
          </li>
        );
      }}
      renderTags={(value) => {
        const displayText = value.join(", ");
        const content = (
          <Box
            component="span"
            sx={{ flex: "1 1 0", minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
          >
            {displayText}
          </Box>
        );
        return value.length === 1 ? content : (
          <Tooltip title={displayText} placement="top">{content}</Tooltip>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={values.length ? undefined : "Search tags…"}
        />
      )}
    />
  );
}
