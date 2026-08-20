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
  Alert,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Check, TriangleAlert, Info } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type { BeCaseType, BeEngagementType } from "@api/backend/types";
import type { Severity, SeverityOrUnset } from "@features/csm-dashboard/types/abtDashboard";
import { SEVERITY_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import {
  ALWAYS_RETAINED_FIELDS,
  caseTypeTransferLabel,
  computeTransferPreview,
  SUPPORTED_TRANSFER_TARGETS,
  TRANSFERABLE_CASE_TYPES,
} from "@features/csm-cases/utils/caseTypeTransfer";

const SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];

const ENGAGEMENT_TYPES: { value: BeEngagementType; label: string }[] = [
  { value: "migration", label: "Migration" },
  { value: "consultancy", label: "Consultancy" },
  { value: "new_feature_improvement", label: "New feature / improvement" },
  { value: "follow_up", label: "Follow up" },
  { value: "onboarding", label: "Onboarding" },
];

/** What the dialog hands back to the caller on confirm. Only `case` and
 * `engagement` are real submit targets — see `SUPPORTED_TRANSFER_TARGETS`. */
export interface CaseTypeTransferSubmission {
  targetType: "case" | "engagement";
  /** Required when `targetType` is `"engagement"`. */
  engagementType?: BeEngagementType;
  /** Optional data-completeness extra, only offered when `targetType` is `"case"`. */
  severity?: Severity;
}

interface ChangeCaseTypeDialogProps {
  currentType: BeCaseType;
  currentSeverity: SeverityOrUnset;
  hasAttachments: boolean;
  isSubmitting: boolean;
  onClose: () => void;
  onSubmit: (submission: CaseTypeTransferSubmission) => void;
}

export default function ChangeCaseTypeDialog({
  currentType,
  currentSeverity,
  hasAttachments,
  isSubmitting,
  onClose,
  onSubmit,
}: ChangeCaseTypeDialogProps): JSX.Element {
  const targets = TRANSFERABLE_CASE_TYPES.filter((t) => t !== currentType);
  const [targetType, setTargetType] = useState<BeCaseType>(targets[0]);
  const [engagementType, setEngagementType] = useState<BeEngagementType | "">("");
  const [severity, setSeverity] = useState<Severity | "">(
    currentSeverity === "unset" ? "" : currentSeverity,
  );

  const preview = computeTransferPreview(currentType, targetType, hasAttachments);
  const isSupportedTarget = SUPPORTED_TRANSFER_TARGETS.includes(targetType);
  const engagementTypeMissing = targetType === "engagement" && engagementType === "";
  const canSubmit = isSupportedTarget && !engagementTypeMissing && !isSubmitting;

  const handleTargetChange = (next: BeCaseType): void => {
    setTargetType(next);
    setEngagementType("");
  };

  const handleConfirm = (): void => {
    if (!canSubmit) return;
    if (targetType === "engagement") {
      onSubmit({ targetType: "engagement", engagementType: engagementType as BeEngagementType });
    } else {
      onSubmit({ targetType: "case", severity: severity === "" ? undefined : severity });
    }
  };

  return (
    <Dialog open onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1.5 }}>
          <span>Change case type</span>
          <Chip size="small" label={`Currently ${caseTypeTransferLabel(currentType)}`} />
        </Box>
      </DialogTitle>
      <DialogContent dividers>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "220px 1fr" },
            gap: 3,
          }}
        >
          <Box>
            <Typography variant="overline" color="text.secondary" sx={{ display: "block", mb: 1 }}>
              Transfer to
            </Typography>
            <Stack spacing={1}>
              {targets.map((t) => {
                const supported = SUPPORTED_TRANSFER_TARGETS.includes(t);
                return (
                  <Tooltip
                    key={t}
                    title={supported ? "" : "Not yet available — needs additional backend support."}
                  >
                    <Button
                      data-testid={`transfer-target-${t}`}
                      variant={targetType === t ? "contained" : "outlined"}
                      color={targetType === t ? "primary" : "inherit"}
                      onClick={() => handleTargetChange(t)}
                      sx={{ justifyContent: "flex-start", textTransform: "none" }}
                    >
                      {caseTypeTransferLabel(t)}
                      {!supported && (
                        <Chip
                          size="small"
                          label="Proposed"
                          sx={{ ml: "auto" }}
                          variant="outlined"
                        />
                      )}
                    </Button>
                  </Tooltip>
                );
              })}
            </Stack>
          </Box>

          <Stack spacing={2} sx={{ minWidth: 0 }}>
            <Box>
              <Typography
                variant="caption"
                sx={{ display: "flex", alignItems: "center", gap: 0.5, color: "success.main", fontWeight: 600 }}
              >
                <Check size={14} /> Retained
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {ALWAYS_RETAINED_FIELDS.join(", ")}.
              </Typography>
            </Box>

            <Box>
              <Typography
                variant="caption"
                sx={{ display: "flex", alignItems: "center", gap: 0.5, color: "warning.main", fontWeight: 600 }}
              >
                <TriangleAlert size={14} /> No longer applies
              </Typography>
              {preview.lostFields.length === 0 ? (
                <Typography variant="body2" color="text.secondary" fontStyle="italic">
                  Nothing type-specific — {caseTypeTransferLabel(currentType)} has no fields beyond
                  the shared ones above.
                </Typography>
              ) : (
                <Stack spacing={0.25}>
                  {preview.lostFields.map((f) => (
                    <Typography key={f.key} variant="body2" color="text.secondary">
                      <strong>{f.label}</strong> — {f.lostReason}
                    </Typography>
                  ))}
                </Stack>
              )}
            </Box>

            <Divider />

            <Box>
              <Typography
                variant="caption"
                sx={{ display: "flex", alignItems: "center", gap: 0.5, color: "primary.main", fontWeight: 600 }}
              >
                <Info size={14} /> Needs to fill in
              </Typography>

              {!isSupportedTarget ? (
                <Alert severity="info" sx={{ mt: 1 }}>
                  Transferring to {caseTypeTransferLabel(targetType)} isn't available yet — the
                  entity-service doesn't accept it as a transfer target. This option is shown to
                  match the full proposal; it can't be submitted until that's extended.
                </Alert>
              ) : targetType === "engagement" ? (
                <FormControl fullWidth size="small" required sx={{ mt: 1 }} error={engagementTypeMissing}>
                  <InputLabel id="transfer-engagement-type-label">Engagement type</InputLabel>
                  <Select
                    labelId="transfer-engagement-type-label"
                    label="Engagement type"
                    value={engagementType}
                    onChange={(e) => setEngagementType(e.target.value as BeEngagementType)}
                  >
                    {ENGAGEMENT_TYPES.map((et) => (
                      <MenuItem key={et.value} value={et.value}>
                        {et.label}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              ) : (
                <Stack spacing={1.5} sx={{ mt: 1 }}>
                  <FormControl fullWidth size="small">
                    <InputLabel id="transfer-severity-label">Severity (optional)</InputLabel>
                    <Select
                      labelId="transfer-severity-label"
                      label="Severity (optional)"
                      value={severity}
                      onChange={(e) => setSeverity(e.target.value as Severity)}
                    >
                      {SEVERITIES.map((s) => (
                        <MenuItem key={s} value={s}>
                          {s} · {SEVERITY_LABEL[s]}
                        </MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                  <Typography variant="caption" color="text.secondary">
                    Issue type isn't offered here — there's no way to update it on an existing
                    case yet, only at creation. Not required to complete the transfer.
                  </Typography>
                </Stack>
              )}

              {preview.attachmentNeeded && (
                <Alert severity="warning" sx={{ mt: 1.5 }}>
                  {caseTypeTransferLabel(targetType)} requires at least one attachment, and this
                  case currently has none. Add one from the Attachments tab before transferring.
                </Alert>
              )}
            </Box>

            <Alert severity="warning" icon={false}>
              SLA targets are type-specific in ServiceNow. This case's SLA clock will be
              recalculated against {caseTypeTransferLabel(targetType)}&rsquo;s policy once
              transferred — the current countdown won&rsquo;t carry over as-is.
            </Alert>
          </Stack>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} color="inherit">
          Cancel
        </Button>
        <Button
          data-testid="transfer-confirm-button"
          variant="contained"
          disabled={!canSubmit}
          loading={isSubmitting}
          onClick={handleConfirm}
        >
          Transfer to {caseTypeTransferLabel(targetType)}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
