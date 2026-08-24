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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Tabs,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Pencil, Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import type {
  BeDeployedProduct,
  BeDeployedProductDetailUpdatePayload,
  BeProductUpdate,
} from "@api/backend/types";

interface EditDeployedProductDialogProps {
  deployedProduct: BeDeployedProduct;
  /** True while the PATCH is in flight; disables actions. */
  isSaving: boolean;
  onClose: () => void;
  /** Persist the changed detail fields (only changed fields are sent). */
  onSave: (payload: BeDeployedProductDetailUpdatePayload) => void;
}

const DESCRIPTION_MAX = 4000;

/** Deep-enough equality for the update-history array (small, plain objects). */
function updatesEqual(a: BeProductUpdate[], b: BeProductUpdate[]): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

/**
 * Edit a deployed product: its cores/tps/description (Details tab) and its
 * update-level history (Update History tab), both via
 * `PATCH /deployments/{deploymentId}/products/{productId}` (detail variant).
 *
 * Only changed fields are sent — the BE requires minProperties 1, so Save is
 * disabled until at least one field differs across either tab. The Update
 * History tab is a client-side array editor: add/edit/delete a row locally,
 * then Save PATCHes the *whole* resulting array (there is no per-entry
 * endpoint) alongside any changed detail fields.
 *
 * Deactivation is a separate concern handled via a confirm dialog in
 * {@link DeployedProductsPanel}, not here — keeping the two BE shapes distinct
 * avoids any chance of accidentally mixing `active` with detail fields.
 *
 * Mount only while open.
 */
export default function EditDeployedProductDialog({
  deployedProduct,
  isSaving,
  onClose,
  onSave,
}: EditDeployedProductDialogProps): JSX.Element {
  const [tab, setTab] = useState(0);

  // --- Details tab state -----------------------------------------------
  // `description` is not in the read schema (DeployedProduct in openapi.yaml
  // does not carry it back) — initialize as empty so we can only set/clear it.
  const originalCores = deployedProduct.cores ?? null;
  const originalTps = deployedProduct.tps ?? null;
  const originalDescription = "";

  const [cores, setCores] = useState(originalCores === null ? "" : String(originalCores));
  const [tps, setTps] = useState(originalTps === null ? "" : String(originalTps));
  const [description, setDescription] = useState(originalDescription);

  const coresNum = cores.trim() === "" ? null : Number(cores);
  const tpsNum = tps.trim() === "" ? null : Number(tps);
  const coresError =
    cores.trim() !== "" &&
    (!Number.isInteger(coresNum) || (coresNum as number) < 0);
  const tpsError =
    tps.trim() !== "" &&
    (isNaN(tpsNum as number) || (tpsNum as number) < 0);

  const coresChanged = coresNum !== originalCores;
  const tpsChanged = tpsNum !== originalTps;
  const descriptionChanged = description.trim() !== originalDescription;

  // --- Update History tab state -----------------------------------------
  const originalUpdates = useMemo<BeProductUpdate[]>(
    () => deployedProduct.updates ?? [],
    [deployedProduct.updates],
  );
  const [updates, setUpdates] = useState<BeProductUpdate[]>(originalUpdates);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [rowLevel, setRowLevel] = useState("");
  const [rowDate, setRowDate] = useState("");
  const [rowDetails, setRowDetails] = useState("");

  const updatesChanged = !updatesEqual(updates, originalUpdates);

  const rowLevelNum = rowLevel.trim() === "" ? null : Number(rowLevel);
  const rowLevelError =
    rowLevel.trim() !== "" && (!Number.isInteger(rowLevelNum) || (rowLevelNum as number) < 0);
  const canAddOrSaveRow =
    rowLevel.trim() !== "" && !rowLevelError && rowDate.trim() !== "";

  const resetRowForm = (): void => {
    setEditingIndex(null);
    setRowLevel("");
    setRowDate("");
    setRowDetails("");
  };

  const handleAddOrSaveRow = (): void => {
    if (!canAddOrSaveRow) return;
    const entry: BeProductUpdate = {
      updateLevel: rowLevelNum as number,
      date: rowDate,
      details: rowDetails.trim() ? rowDetails.trim() : undefined,
    };
    if (editingIndex === null) {
      setUpdates((prev) => [...prev, entry]);
    } else {
      setUpdates((prev) => prev.map((u, i) => (i === editingIndex ? entry : u)));
    }
    resetRowForm();
  };

  const handleEditRow = (index: number): void => {
    const u = updates[index];
    setEditingIndex(index);
    setRowLevel(String(u.updateLevel));
    setRowDate(u.date);
    setRowDetails(u.details ?? "");
  };

  const handleDeleteRow = (index: number): void => {
    setUpdates((prev) => prev.filter((_, i) => i !== index));
    if (editingIndex === index) resetRowForm();
  };

  // --- Combined save ------------------------------------------------------
  const payload = useMemo<BeDeployedProductDetailUpdatePayload>(() => {
    const next: Record<string, unknown> = {};
    if (coresChanged) next.cores = coresNum;
    if (tpsChanged) next.tps = tpsNum;
    if (descriptionChanged) {
      next.description = description.trim().length > 0 ? description.trim() : null;
    }
    if (updatesChanged) next.updates = updates;
    return next as BeDeployedProductDetailUpdatePayload;
  }, [
    coresChanged,
    tpsChanged,
    descriptionChanged,
    coresNum,
    tpsNum,
    description,
    updatesChanged,
    updates,
  ]);

  const canSave =
    !isSaving &&
    !coresError &&
    !tpsError &&
    (coresChanged || tpsChanged || descriptionChanged || updatesChanged);

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Edit deployed product</DialogTitle>
      <Tabs value={tab} onChange={(_e, v: number) => setTab(v)} sx={{ px: 3, minHeight: 36 }}>
        <Tab label="Details" id="edit-deployed-product-tab-details" />
        <Tab label="Update History" id="edit-deployed-product-tab-history" />
      </Tabs>
      <DialogContent dividers>
        {tab === 0 && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
            <TextField
              label="Cores"
              value={cores}
              onChange={(e) => setCores(e.target.value)}
              size="small"
              fullWidth
              type="number"
              autoFocus
              slotProps={{ htmlInput: { min: 0, step: 1 } }}
              error={coresError}
              helperText={coresError ? "Must be a non-negative integer." : " "}
            />

            <TextField
              label="TPS"
              value={tps}
              onChange={(e) => setTps(e.target.value)}
              size="small"
              fullWidth
              type="number"
              slotProps={{ htmlInput: { min: 0, step: 0.1 } }}
              error={tpsError}
              helperText={tpsError ? "Must be a non-negative number." : " "}
            />

            <TextField
              label="Description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              size="small"
              fullWidth
              multiline
              minRows={2}
              slotProps={{ htmlInput: { maxLength: DESCRIPTION_MAX } }}
            />
          </Box>
        )}

        {tab === 1 && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
            {updates.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                No update history recorded.
              </Typography>
            ) : (
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>Level</TableCell>
                    <TableCell>Date</TableCell>
                    <TableCell>Details</TableCell>
                    <TableCell align="right">Actions</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {updates.map((u, i) => (
                    <TableRow key={i}>
                      <TableCell>{u.updateLevel}</TableCell>
                      <TableCell>{u.date}</TableCell>
                      <TableCell sx={{ maxWidth: 160 }}>
                        <Typography variant="body2" noWrap title={u.details ?? undefined}>
                          {u.details || "—"}
                        </Typography>
                      </TableCell>
                      <TableCell align="right">
                        <IconButton
                          size="small"
                          aria-label={`Edit update level ${u.updateLevel}`}
                          onClick={() => handleEditRow(i)}
                          disabled={isSaving}
                        >
                          <Pencil size={14} />
                        </IconButton>
                        <IconButton
                          size="small"
                          aria-label={`Delete update level ${u.updateLevel}`}
                          onClick={() => handleDeleteRow(i)}
                          disabled={isSaving}
                        >
                          <Trash2 size={14} />
                        </IconButton>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            <Box
              sx={{
                display: "flex",
                flexDirection: "column",
                gap: 1.5,
                border: 1,
                borderColor: "divider",
                borderRadius: 1,
                p: 1.5,
              }}
            >
              <Typography variant="caption" color="text.secondary">
                {editingIndex === null ? "Add an update" : "Edit update"}
              </Typography>
              <Box sx={{ display: "flex", gap: 1.5, flexWrap: "wrap" }}>
                <TextField
                  label="Update level"
                  value={rowLevel}
                  onChange={(e) => setRowLevel(e.target.value)}
                  size="small"
                  type="number"
                  slotProps={{ htmlInput: { min: 0, step: 1 } }}
                  error={rowLevelError}
                  helperText={rowLevelError ? "Must be a non-negative integer." : " "}
                  sx={{ flex: 1, minWidth: 120 }}
                />
                <TextField
                  label="Date"
                  value={rowDate}
                  onChange={(e) => setRowDate(e.target.value)}
                  size="small"
                  type="date"
                  slotProps={{ inputLabel: { shrink: true } }}
                  sx={{ flex: 1, minWidth: 160 }}
                />
              </Box>
              <TextField
                label="Details"
                value={rowDetails}
                onChange={(e) => setRowDetails(e.target.value)}
                size="small"
                fullWidth
                multiline
                minRows={2}
              />
              <Box sx={{ display: "flex", gap: 1, justifyContent: "flex-end" }}>
                {editingIndex !== null && (
                  <Button size="small" onClick={resetRowForm} disabled={isSaving}>
                    Cancel
                  </Button>
                )}
                <Button
                  size="small"
                  variant="outlined"
                  startIcon={<Plus size={14} />}
                  onClick={handleAddOrSaveRow}
                  disabled={!canAddOrSaveRow || isSaving}
                >
                  {editingIndex === null ? "Add" : "Save row"}
                </Button>
              </Box>
            </Box>
          </Box>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button variant="contained" disabled={!canSave} onClick={() => onSave(payload)}>
          Save changes
        </Button>
      </DialogActions>
    </Dialog>
  );
}
