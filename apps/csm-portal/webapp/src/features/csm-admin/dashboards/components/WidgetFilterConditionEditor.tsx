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
  Autocomplete,
  Box,
  Button,
  IconButton,
  MenuItem,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { BeWidgetResourceType } from "@api/backend/types";
import {
  CASE_FIELD_OPTIONS,
  operatorsForResourceType,
  usesCaseFieldFilterDsl,
  type FilterCondition,
  type FilterConditionOp,
} from "@features/csm-admin/dashboards/utils/widgetQueryConditions";

const OP_LABEL: Record<FilterConditionOp, string> = {
  eq: "is",
  in: "is any of",
  notIn: "is none of",
  gte: "is on/after (≥)",
  lte: "is on/before (≤)",
  isEmpty: "is empty",
  isNotEmpty: "is not empty",
};

const NO_VALUE_OPS = new Set<FilterConditionOp>(["isEmpty", "isNotEmpty"]);

interface WidgetFilterConditionEditorProps {
  resourceType: BeWidgetResourceType;
  conditions: FilterCondition[];
  onChange: (next: FilterCondition[]) => void;
}

/**
 * The widget editor's filter builder: one row per condition (field,
 * operator, value(s)), rather than a raw JSON textarea — see
 * `widgetQueryConditions.ts` for how a row round-trips through whichever of
 * the app's two real filter shapes this widget's `resourceType` actually
 * needs.
 */
export default function WidgetFilterConditionEditor({
  resourceType,
  conditions,
  onChange,
}: WidgetFilterConditionEditorProps): JSX.Element {
  const isCaseLike = usesCaseFieldFilterDsl(resourceType);
  const fieldOptions = isCaseLike ? CASE_FIELD_OPTIONS : [];
  // Only offer operators this resourceType's own search contract can
  // actually express (see `operatorsForResourceType`'s doc comment) — a
  // non-case resourceType has no generic notIn/gte/lte/isEmpty/isNotEmpty
  // convention, so offering them here would let the admin build a filter
  // this app cannot serialize correctly.
  const availableOps = operatorsForResourceType(resourceType);

  const updateRow = (index: number, patch: Partial<FilterCondition>): void => {
    const next = conditions.map((c, i) => (i === index ? { ...c, ...patch } : c));
    onChange(next);
  };

  const removeRow = (index: number): void => {
    onChange(conditions.filter((_, i) => i !== index));
  };

  const addRow = (): void => {
    onChange([...conditions, { field: "", op: "eq", values: [] }]);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      {conditions.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          No filters — this widget matches every {resourceType.replace(/_/g, " ")} record.
        </Typography>
      )}
      {conditions.map((condition, index) => {
        // A row whose op isn't in this resourceType's own supported list
        // (only possible from data written before that restriction existed,
        // or a resourceType switch elsewhere clearing conditions
        // notwithstanding) still needs its own current value represented in
        // the Select, or MUI renders it blank — offered alongside the real
        // list rather than silently swapped out from under the admin.
        const rowOps = availableOps.includes(condition.op)
          ? availableOps
          : [...availableOps, condition.op];
        return (
          <Box
            key={index}
            sx={{
              display: "flex",
              flexWrap: "wrap",
              alignItems: "flex-start",
              gap: 1,
            }}
          >
            <Autocomplete
              freeSolo
              size="small"
              options={fieldOptions}
              value={condition.field}
              onInputChange={(_e, value) => updateRow(index, { field: value })}
              sx={{ minWidth: 180, flex: "1 1 180px" }}
              renderInput={(params) => (
                <TextField
                  {...params}
                  label="Field"
                  slotProps={{ htmlInput: { ...params.inputProps, "aria-label": "Filter field" } }}
                />
              )}
            />
            <TextField
              select
              size="small"
              label="Operator"
              value={condition.op}
              onChange={(e) => updateRow(index, { op: e.target.value as FilterConditionOp })}
              sx={{ minWidth: 160 }}
            >
              {rowOps.map((op) => (
                <MenuItem key={op} value={op}>
                  {OP_LABEL[op]}
                </MenuItem>
              ))}
            </TextField>
            {!NO_VALUE_OPS.has(condition.op) && (
              <Autocomplete
                multiple
                freeSolo
                size="small"
                options={[]}
                value={condition.values}
                onChange={(_e, next) =>
                  updateRow(index, { values: next.map((v) => v.trim()).filter((v) => v.length > 0) })
                }
                sx={{ minWidth: 220, flex: "2 1 220px" }}
                renderInput={(params) => (
                  <TextField
                    {...params}
                    label="Value(s)"
                    placeholder={condition.values.length ? undefined : "Type a value and press Enter…"}
                    slotProps={{ htmlInput: { ...params.inputProps, "aria-label": "Filter value" } }}
                  />
                )}
              />
            )}
            <Tooltip title="Remove this filter">
              <IconButton size="small" onClick={() => removeRow(index)} aria-label="Remove filter">
                <Trash2 size={16} />
              </IconButton>
            </Tooltip>
          </Box>
        );
      })}
      <Button
        size="small"
        variant="text"
        startIcon={<Plus size={16} />}
        onClick={addRow}
        sx={{ alignSelf: "flex-start" }}
      >
        Add filter
      </Button>
    </Box>
  );
}
