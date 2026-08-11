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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  MenuItem,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Eye, Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import type {
  BeDashboardPieSlice,
  BeDashboardWidget,
  BeWidgetPaletteColor,
  BeWidgetResourceType,
  BeWidgetShape,
} from "@api/backend/types";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import DashboardWidgetTile from "@features/csm-dashboard/components/DashboardWidgetTile";
import WidgetFilterConditionEditor from "@features/csm-admin/dashboards/components/WidgetFilterConditionEditor";
import { newWidgetId } from "@features/csm-admin/dashboards/utils/dashboardDraftsStorage";
import {
  filterConditionsFromQuery,
  queryFromFilterConditions,
  type FilterCondition,
} from "@features/csm-admin/dashboards/utils/widgetQueryConditions";

const RESOURCE_TYPES = Object.keys(WIDGET_RESOURCE_CONFIG) as BeWidgetResourceType[];
const SHAPES: BeWidgetShape[] = ["count", "list", "pie", "bar"];
const PALETTE_COLORS: BeWidgetPaletteColor[] = [
  "primary",
  "secondary",
  "success",
  "error",
  "info",
  "warning",
];

interface SliceDraft {
  label: string;
  color?: BeWidgetPaletteColor;
  conditions: FilterCondition[];
}

function slicesToDrafts(resourceType: BeWidgetResourceType, slices: BeDashboardPieSlice[] | undefined): SliceDraft[] {
  return (slices ?? []).map((s) => ({
    label: s.label,
    color: s.color,
    conditions: filterConditionsFromQuery(resourceType, s.query),
  }));
}

function draftsToSlices(resourceType: BeWidgetResourceType, drafts: SliceDraft[]): BeDashboardPieSlice[] {
  return drafts
    .filter((d) => d.label.trim().length > 0)
    .map((d) => ({
      label: d.label,
      color: d.color,
      query: queryFromFilterConditions(resourceType, d.conditions),
    }));
}

interface WidgetEditorDialogProps {
  /** `undefined` when creating a brand-new widget. */
  widget: BeDashboardWidget | undefined;
  /** Pre-fills the section field for a brand-new widget created via a
   * specific section's own "Add widget" action (see the editor page) —
   * ignored when `widget` is set (editing keeps that widget's own
   * section). */
  defaultSection?: string;
  /** Existing section names on this dashboard draft, offered as
   * autocomplete suggestions (freeform text is still accepted — a widget
   * can also start a brand-new section right here). */
  sectionSuggestions: string[];
  /** The team the Preview tile below should scope its data to, threaded
   * through exactly as `DashboardWidgetGrid` threads it to every real tile
   * (see that component's own doc comment) — otherwise a widget using the
   * `__current_team__` filter placeholder or a `{{currentTeam}}` display-text
   * token previews unfiltered data / an unresolved placeholder instead of
   * what an admin would actually see on the live dashboard. `undefined` for
   * a non-team-based dashboard, or while the team isn't resolved yet — see
   * the editor page's own doc comment for where this comes from. */
  selectedTeamGroupId?: string | string[];
  /** See `selectedTeamGroupId` above; the human-readable counterpart for the
   * `{{currentTeam}}` text token — see `DashboardWidgetGrid`. */
  selectedTeamLabel?: string;
  onClose: () => void;
  onSave: (widget: BeDashboardWidget) => void;
  onDelete?: () => void;
}

/**
 * Modal editor for a single dashboard widget: a form for everything a
 * `BeDashboardWidget` carries (display metadata, resourceType/shape,
 * filters, and shape-specific fields), plus a "Preview" button that renders
 * the in-progress config through the exact same `DashboardWidgetTile` the
 * live dashboard (and this builder's own grid) render with — so a "run the
 * current draft through the real resolution path" preview needs no
 * parallel fetch/render logic of its own.
 */
export default function WidgetEditorDialog({
  widget,
  defaultSection,
  sectionSuggestions,
  selectedTeamGroupId,
  selectedTeamLabel,
  onClose,
  onSave,
  onDelete,
}: WidgetEditorDialogProps): JSX.Element {
  const isNew = widget === undefined;
  const [widgetId] = useState(() => widget?.widgetId ?? newWidgetId());
  const [displayName, setDisplayName] = useState(widget?.displayName ?? "");
  const [description, setDescription] = useState(widget?.description ?? "");
  const [resourceType, setResourceType] = useState<BeWidgetResourceType>(
    widget?.resourceType ?? "case",
  );
  const [shape, setShape] = useState<BeWidgetShape>(widget?.shape ?? "count");
  const [section, setSection] = useState(widget?.section ?? defaultSection ?? "");
  const [gridWidth, setGridWidth] = useState(widget?.gridWidth ?? 3);
  const [listLimit, setListLimit] = useState<number | undefined>(widget?.listLimit);
  const [groupBy, setGroupBy] = useState(widget?.groupBy ?? "");
  const [conditions, setConditions] = useState<FilterCondition[]>(() =>
    filterConditionsFromQuery(widget?.resourceType ?? "case", widget?.query),
  );
  const [sliceDrafts, setSliceDrafts] = useState<SliceDraft[]>(() =>
    slicesToDrafts(widget?.resourceType ?? "case", widget?.slices),
  );
  // A resourceType switch invalidates the previous filter shape entirely
  // (see widgetQueryConditions.ts's own doc comment) — rather than silently
  // reinterpreting stale rows against a contract they were never written
  // for, clear them and let the admin rebuild for the new resourceType.
  const handleResourceTypeChange = (next: BeWidgetResourceType): void => {
    setResourceType(next);
    setConditions([]);
    setSliceDrafts((prev) => prev.map((d) => ({ ...d, conditions: [] })));
  };

  const [previewSnapshot, setPreviewSnapshot] = useState<BeDashboardWidget | undefined>();

  const canSave = displayName.trim().length > 0 && gridWidth >= 1 && gridWidth <= 12;

  const buildWidget = (): BeDashboardWidget => ({
    widgetId,
    displayName: displayName.trim(),
    description: description.trim() || undefined,
    resourceType,
    shape,
    gridWidth,
    query: queryFromFilterConditions(resourceType, conditions),
    section: section.trim() || undefined,
    groupBy: groupBy.trim() || undefined,
    listLimit: shape === "list" ? listLimit : undefined,
    slices: shape === "pie" || shape === "bar" ? draftsToSlices(resourceType, sliceDrafts) : undefined,
  });

  const handlePreview = (): void => setPreviewSnapshot(buildWidget());

  const handleSave = (): void => {
    if (!canSave) return;
    onSave(buildWidget());
  };

  const addSlice = (): void => setSliceDrafts((prev) => [...prev, { label: "", conditions: [] }]);
  const removeSlice = (index: number): void =>
    setSliceDrafts((prev) => prev.filter((_, i) => i !== index));
  const updateSlice = (index: number, patch: Partial<SliceDraft>): void =>
    setSliceDrafts((prev) => prev.map((d, i) => (i === index ? { ...d, ...patch } : d)));

  const isChartShape = shape === "pie" || shape === "bar";
  const previewKey = useMemo(
    () => (previewSnapshot ? JSON.stringify(previewSnapshot) : undefined),
    [previewSnapshot],
  );

  return (
    <Dialog open onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>{isNew ? "Add widget" : "Edit widget"}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <TextField
              label="Display name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
              size="small"
              sx={{ flex: "1 1 260px" }}
              slotProps={{ htmlInput: { "aria-label": "Widget display name" } }}
            />
            <TextField
              label="Description (optional)"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              size="small"
              sx={{ flex: "1 1 260px" }}
            />
          </Box>

          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <TextField
              select
              label="Resource type"
              value={resourceType}
              onChange={(e) => handleResourceTypeChange(e.target.value as BeWidgetResourceType)}
              size="small"
              sx={{ minWidth: 200 }}
            >
              {RESOURCE_TYPES.map((rt) => (
                <MenuItem key={rt} value={rt}>
                  {rt.replace(/_/g, " ")}
                </MenuItem>
              ))}
            </TextField>
            <TextField
              select
              label="Shape"
              value={shape}
              onChange={(e) => setShape(e.target.value as BeWidgetShape)}
              size="small"
              sx={{ minWidth: 160 }}
            >
              {SHAPES.map((s) => (
                <MenuItem key={s} value={s}>
                  {s}
                </MenuItem>
              ))}
            </TextField>
            <Autocomplete
              freeSolo
              size="small"
              options={sectionSuggestions}
              value={section}
              onInputChange={(_e, value) => setSection(value)}
              sx={{ minWidth: 200, flex: "1 1 200px" }}
              renderInput={(params) => (
                <TextField {...params} label="Section (optional)" placeholder="Untitled group" />
              )}
            />
            <TextField
              label="Grid width (1–12)"
              type="number"
              value={gridWidth}
              onChange={(e) => setGridWidth(Math.min(12, Math.max(1, Number(e.target.value) || 1)))}
              size="small"
              sx={{ width: 160 }}
              slotProps={{ htmlInput: { min: 1, max: 12 } }}
            />
            {shape === "list" && (
              <TextField
                label="Row limit (optional)"
                type="number"
                value={listLimit ?? ""}
                onChange={(e) => {
                  const raw = e.target.value;
                  if (raw === "") {
                    setListLimit(undefined);
                    return;
                  }
                  const parsed = Number(raw);
                  // Same clamp `gridWidth` above applies, plus a guard
                  // `gridWidth` doesn't need: an invalid (non-numeric, e.g.
                  // pasted text) keystroke is ignored outright rather than
                  // written through as `NaN` — `JSON.stringify`s a `NaN` to
                  // `null` in the deployable widget JSON, silently corrupting
                  // it. Falls back to the previous valid value, not a
                  // default, since "no explicit limit" (`undefined`) is
                  // already reachable via the empty-string case above.
                  if (Number.isFinite(parsed)) setListLimit(Math.max(1, Math.trunc(parsed)));
                }}
                size="small"
                sx={{ width: 180 }}
                slotProps={{ htmlInput: { min: 1 } }}
              />
            )}
            <TextField
              label="Group by (optional)"
              value={groupBy}
              onChange={(e) => setGroupBy(e.target.value)}
              size="small"
              sx={{ minWidth: 180 }}
              helperText="Present on the wire; not used by the frontend today."
            />
          </Box>

          <Divider />

          <Typography variant="subtitle2">Filters</Typography>
          <WidgetFilterConditionEditor
            resourceType={resourceType}
            conditions={conditions}
            onChange={setConditions}
          />

          {isChartShape && (
            <>
              <Divider />
              <Typography variant="subtitle2">
                Slices — one search per slice, each merged under the filters above
              </Typography>
              {sliceDrafts.map((slice, index) => (
                <Box
                  key={index}
                  sx={{ border: 1, borderColor: "divider", borderRadius: 1, p: 1.5, display: "flex", flexDirection: "column", gap: 1 }}
                >
                  <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
                    <TextField
                      label="Slice label"
                      value={slice.label}
                      onChange={(e) => updateSlice(index, { label: e.target.value })}
                      size="small"
                      sx={{ flex: "1 1 200px" }}
                    />
                    <TextField
                      select
                      label="Color (optional)"
                      value={slice.color ?? ""}
                      onChange={(e) =>
                        updateSlice(index, {
                          color: (e.target.value || undefined) as BeWidgetPaletteColor | undefined,
                        })
                      }
                      size="small"
                      sx={{ minWidth: 160 }}
                    >
                      <MenuItem value="">Default rotation</MenuItem>
                      {PALETTE_COLORS.map((c) => (
                        <MenuItem key={c} value={c}>
                          {c}
                        </MenuItem>
                      ))}
                    </TextField>
                    <IconButton
                      size="small"
                      aria-label={`Remove slice ${slice.label || index + 1}`}
                      onClick={() => removeSlice(index)}
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </Box>
                  <WidgetFilterConditionEditor
                    resourceType={resourceType}
                    conditions={slice.conditions}
                    onChange={(next) => updateSlice(index, { conditions: next })}
                  />
                </Box>
              ))}
              <Button size="small" variant="text" startIcon={<Plus size={16} />} onClick={addSlice} sx={{ alignSelf: "flex-start" }}>
                Add slice
              </Button>
            </>
          )}

          <Divider />

          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <Typography variant="subtitle2">Preview</Typography>
            <Button size="small" variant="outlined" startIcon={<Eye size={14} />} onClick={handlePreview}>
              Preview
            </Button>
          </Box>
          {previewSnapshot ? (
            <Box sx={{ maxWidth: 420 }}>
              <DashboardWidgetTile
                key={previewKey}
                widgetId={previewSnapshot.widgetId}
                displayName={previewSnapshot.displayName}
                description={previewSnapshot.description}
                resourceType={previewSnapshot.resourceType}
                shape={previewSnapshot.shape}
                filters={previewSnapshot.query}
                listLimit={previewSnapshot.listLimit}
                slices={previewSnapshot.slices}
                sortBy={previewSnapshot.sortBy}
                selectedTeamGroupId={selectedTeamGroupId}
                selectedTeamLabel={selectedTeamLabel}
              />
            </Box>
          ) : (
            <Typography variant="body2" color="text.secondary">
              Click "Preview" to run this widget's current settings against real data before
              saving.
            </Typography>
          )}
        </Box>
      </DialogContent>
      <DialogActions sx={{ justifyContent: "space-between", px: 3, pb: 2 }}>
        <Box>
          {!isNew && onDelete && (
            <Button color="error" onClick={onDelete}>
              Delete widget
            </Button>
          )}
        </Box>
        <Box sx={{ display: "flex", gap: 1 }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="contained" disabled={!canSave} onClick={handleSave}>
            {isNew ? "Add widget" : "Save widget"}
          </Button>
        </Box>
      </DialogActions>
    </Dialog>
  );
}
