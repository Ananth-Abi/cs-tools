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
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import MultiSelectField from "@components/MultiSelectField";
import {
  ADVANCED_FILTER_FIELDS,
  defaultAdvancedFilterRow,
  getAdvancedFilterFieldMeta,
  getAdvancedFilterOpMeta,
  type AdvancedFilterField,
  type AdvancedFilterRow,
} from "@features/csm-cases/utils/advancedFilters";

interface AdvancedFiltersBuilderProps {
  rows: AdvancedFilterRow[];
  onChange: (next: AdvancedFilterRow[]) => void;
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
                  options={fieldMeta?.options ?? []}
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
              {opMeta?.valueKind === "date" && (
                <TextField
                  size="small"
                  fullWidth
                  label="Date"
                  placeholder={fieldMeta?.placeholder ?? "YYYY-MM-DD"}
                  value={row.values[0] ?? ""}
                  onChange={(e) =>
                    updateRow(index, { ...row, values: e.target.value ? [e.target.value] : [] })
                  }
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
