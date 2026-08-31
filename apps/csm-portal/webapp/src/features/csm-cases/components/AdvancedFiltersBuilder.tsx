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
  AdapterDateFns,
  Box,
  Button,
  DatePickers,
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import MultiSelectField from "@components/MultiSelectField";
import AsyncCreatedByMultiSelect from "@features/csm-cases/components/AsyncCreatedByMultiSelect";
import {
  ADVANCED_FILTER_FIELDS,
  RELATIVE_DATE_PRESETS,
  defaultAdvancedFilterRow,
  getAdvancedFilterFieldMeta,
  getAdvancedFilterOpMeta,
  type AdvancedFilterField,
  type AdvancedFilterRow,
} from "@features/csm-cases/utils/advancedFilters";

const { DatePicker, LocalizationProvider } = DatePickers;

interface AdvancedFiltersBuilderProps {
  rows: AdvancedFilterRow[];
  onChange: (next: AdvancedFilterRow[]) => void;
  /** SRE team options (`sreGroupId` → display name), computed once in
   * `CasesFilterBar.tsx` from the same `useTeams(true)` fetch the "Team"
   * (`creTeam`) bar control uses — the `sreTeam` row's value kind is
   * `multiSelect`, but its options are fetched data, not a fixed enum, so
   * they're threaded in here rather than living in the static
   * `advancedFilters.ts` catalogue. */
  sreTeamOptions: { value: string; label: string }[];
}

/** "YYYY-MM-DD" to a local-midnight Date (avoids the UTC-parse day-shift
 * `new Date(dateString)` can cause depending on the viewer's timezone) —
 * same helper `DateRangeFilter`/`ChangeRequestsFilterBar` each keep locally
 * for their own date-only fields; duplicated here for the same reason
 * `DateRangeFilter` duplicates it rather than importing across features. */
function parseDateOnly(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Local-midnight Date back to "YYYY-MM-DD". */
function formatDateOnly(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

const CUSTOM_DATE_SENTINEL = "__custom_date__";

function isRelativeDatePreset(value: string): boolean {
  return RELATIVE_DATE_PRESETS.some((p) => p.value === value);
}

interface DateOrPresetValueInputProps {
  /** A relative-date placeholder (one of `RELATIVE_DATE_PRESETS`), a literal
   * `YYYY-MM-DD`, or `""` (nothing chosen yet). */
  value: string;
  onChange: (next: string) => void;
}

/**
 * The `createdOn`/`updatedOn`/`closedOn` row's value input: a preset
 * dropdown (human labels for the common relative-date placeholders — see
 * `RELATIVE_DATE_PRESETS`) plus an actual calendar date picker for an exact
 * day, so neither the placeholder grammar (`__daysAgo:N__`, ...) nor a raw
 * `YYYY-MM-DD` ever has to be hand-typed. Mode (`preset` vs `custom`) is
 * local state seeded from the incoming value, since a bare string can't
 * distinguish "no date chosen yet" from "chose Custom, haven't picked a day
 * yet" — the caller should key this component by `field-op` (see
 * `AdvancedFiltersBuilder`) so switching to a different date row/op resets
 * that local state instead of carrying it over.
 */
function DateOrPresetValueInput({ value, onChange }: DateOrPresetValueInputProps): JSX.Element {
  const [mode, setMode] = useState<"preset" | "custom">(
    value && !isRelativeDatePreset(value) ? "custom" : "preset",
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <FormControl size="small" fullWidth>
        <InputLabel id="advanced-filter-date-mode-label">Date</InputLabel>
        <Select
          labelId="advanced-filter-date-mode-label"
          label="Date"
          value={mode === "custom" ? CUSTOM_DATE_SENTINEL : value}
          displayEmpty
          onChange={(e) => {
            const next = e.target.value;
            if (next === CUSTOM_DATE_SENTINEL) {
              setMode("custom");
              onChange("");
            } else {
              setMode("preset");
              onChange(next);
            }
          }}
        >
          <MenuItem value="">
            <em>Choose…</em>
          </MenuItem>
          {RELATIVE_DATE_PRESETS.map((p) => (
            <MenuItem key={p.value} value={p.value}>
              {p.label}
            </MenuItem>
          ))}
          <MenuItem value={CUSTOM_DATE_SENTINEL}>Custom date…</MenuItem>
        </Select>
      </FormControl>
      {mode === "custom" && (
        <LocalizationProvider dateAdapter={AdapterDateFns}>
          <DatePicker
            label="Exact date"
            value={parseDateOnly(value)}
            onChange={(date) =>
              onChange(
                date instanceof Date && !Number.isNaN(date.getTime())
                  ? formatDateOnly(date)
                  : "",
              )
            }
            slotProps={{
              textField: { size: "small", fullWidth: true },
              field: { clearable: true },
            }}
          />
        </LocalizationProvider>
      )}
    </Box>
  );
}

/** Splits a comma-separated free-text entry into a trimmed, non-empty array. */
function splitCsv(raw: string): string[] {
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

/**
 * Ad-hoc "field → operator → value" filter row builder, additive to the
 * dedicated bar controls (severity, state, case type, work state, engagement
 * type, assignee, project, product) above it — those already cover their own
 * fields well and are never offered here (see `advancedFilters.ts`'s field
 * catalogue). Each row becomes one extra `BeCaseFieldFilter` entry in the
 * `/cases/search` payload (see `caseSearchPayload.ts`).
 */
export default function AdvancedFiltersBuilder({
  rows,
  onChange,
  sreTeamOptions,
}: AdvancedFiltersBuilderProps): JSX.Element {
  const updateRow = (index: number, next: AdvancedFilterRow): void => {
    onChange(rows.map((r, i) => (i === index ? next : r)));
  };
  const removeRow = (index: number): void => {
    onChange(rows.filter((_, i) => i !== index));
  };
  const addRow = (): void => {
    onChange([...rows, defaultAdvancedFilterRow()]);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Typography variant="subtitle2" color="text.secondary">
        Advanced filters
      </Typography>
      {rows.map((row, index) => {
        const fieldMeta = getAdvancedFilterFieldMeta(row.field);
        const opMeta = getAdvancedFilterOpMeta(row.field, row.op);
        return (
          <Box
            key={`advanced-filter-row-${index}`}
            sx={{ display: "flex", gap: 1, alignItems: "flex-start", flexWrap: "wrap" }}
          >
            <FormControl size="small" sx={{ minWidth: 200 }}>
              <InputLabel id={`advanced-filter-field-${index}-label`}>Field</InputLabel>
              <Select
                labelId={`advanced-filter-field-${index}-label`}
                label="Field"
                value={row.field}
                onChange={(e) => {
                  const nextField = e.target.value as AdvancedFilterField;
                  const nextFieldMeta = getAdvancedFilterFieldMeta(nextField);
                  const nextOp = nextFieldMeta?.ops[0]?.op ?? row.op;
                  updateRow(index, { field: nextField, op: nextOp, values: [] });
                }}
              >
                {ADVANCED_FILTER_FIELDS.map((m) => (
                  <MenuItem key={m.field} value={m.field}>
                    {m.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <FormControl size="small" sx={{ minWidth: 160 }}>
              <InputLabel id={`advanced-filter-op-${index}-label`}>Operator</InputLabel>
              <Select
                labelId={`advanced-filter-op-${index}-label`}
                label="Operator"
                value={row.op}
                onChange={(e) => {
                  updateRow(index, { ...row, op: e.target.value as typeof row.op, values: [] });
                }}
              >
                {(fieldMeta?.ops ?? []).map((o) => (
                  <MenuItem key={o.op} value={o.op}>
                    {o.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <Box sx={{ minWidth: 220, flex: "1 1 220px" }}>
              {opMeta?.valueKind === "multiText" && (
                <TextField
                  size="small"
                  fullWidth
                  label="Value(s)"
                  placeholder={fieldMeta?.placeholder ?? "Comma-separated values"}
                  value={row.values.join(", ")}
                  onChange={(e) => updateRow(index, { ...row, values: splitCsv(e.target.value) })}
                  helperText={
                    fieldMeta?.suggestions?.length
                      ? `Suggestions: ${fieldMeta.suggestions.join(", ")}`
                      : "Comma-separated"
                  }
                />
              )}
              {opMeta?.valueKind === "multiSelect" && (
                <MultiSelectField
                  id={`advanced-filter-value-${index}`}
                  label="Value(s)"
                  values={row.values}
                  // `sreTeam`'s options are fetched data (the team registry),
                  // not part of the static catalogue -- see the
                  // `sreTeamOptions` prop's own doc comment.
                  options={
                    row.field === "sreTeam" ? sreTeamOptions : (fieldMeta?.options ?? [])
                  }
                  onChange={(next) => updateRow(index, { ...row, values: next })}
                />
              )}
              {opMeta?.valueKind === "asyncEmailMultiSelect" && (
                <AsyncCreatedByMultiSelect
                  values={row.values}
                  onChange={(next) => updateRow(index, { ...row, values: next })}
                />
              )}
              {opMeta?.valueKind === "text" && (
                <TextField
                  size="small"
                  fullWidth
                  label="Value"
                  placeholder={fieldMeta?.placeholder}
                  value={row.values[0] ?? ""}
                  onChange={(e) =>
                    updateRow(index, { ...row, values: e.target.value ? [e.target.value] : [] })
                  }
                />
              )}
              {opMeta?.valueKind === "number" && (
                <TextField
                  size="small"
                  fullWidth
                  type="number"
                  label="Value"
                  value={row.values[0] ?? ""}
                  onChange={(e) =>
                    updateRow(index, { ...row, values: e.target.value ? [e.target.value] : [] })
                  }
                />
              )}
              {opMeta?.valueKind === "dateOrPreset" && (
                <DateOrPresetValueInput
                  key={`${row.field}-${row.op}`}
                  value={row.values[0] ?? ""}
                  onChange={(next) => updateRow(index, { ...row, values: next ? [next] : [] })}
                />
              )}
              {(opMeta?.valueKind === "none" || opMeta?.valueKind === "currentUser") && (
                <Typography variant="caption" color="text.secondary" sx={{ lineHeight: "40px" }}>
                  {opMeta.valueKind === "currentUser" ? "The signed-in user" : "No value needed"}
                </Typography>
              )}
            </Box>

            <IconButton
              size="small"
              aria-label="Remove filter row"
              onClick={() => removeRow(index)}
              sx={{ mt: 0.5 }}
            >
              <Trash2 size={16} />
            </IconButton>
          </Box>
        );
      })}
      <Box>
        <Button size="small" variant="outlined" startIcon={<Plus size={16} />} onClick={addRow}>
          Add filter
        </Button>
      </Box>
    </Box>
  );
}
