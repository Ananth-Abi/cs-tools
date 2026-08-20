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
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  InputLabel,
  MenuItem,
  Radio,
  RadioGroup,
  Select,
  Typography,
} from "@wso2/oxygen-ui";
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

/**
 * Transfer a case between Case, Engagement, Security Report Analysis, and
 * Service Request (digiops-cs#2818). Only Case <-> Engagement submits today
 * — SRA/Service Request are still offered so the picker previews what
 * transferring into them would retain/lose, but the confirm button stays
 * disabled until entity-service's `caseType` validator (digiops-cs#2852)
 * accepts them as targets.
 */
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
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Change case type</DialogTitle>
      <DialogContent dividers sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="body2" color="text.secondary">
          Currently{" "}
          <Typography component="span" variant="body2" sx={{ fontWeight: 600, color: "text.primary" }}>
            {caseTypeTransferLabel(currentType)}
          </Typography>
          . {ALWAYS_RETAINED_FIELDS.join(", ")} always carry over.
        </Typography>

        <FormControl>
          <RadioGroup
            value={targetType}
            onChange={(e) => handleTargetChange(e.target.value as BeCaseType)}
          >
            {targets.map((t) => {
              const supported = SUPPORTED_TRANSFER_TARGETS.includes(t);
              return (
                <FormControlLabel
                  key={t}
                  value={t}
                  disabled={isSubmitting}
                  control={<Radio size="small" />}
                  label={
                    supported
                      ? caseTypeTransferLabel(t)
                      : `${caseTypeTransferLabel(t)} (not yet available)`
                  }
                />
              );
            })}
          </RadioGroup>
        </FormControl>

        {preview.lostFields.length > 0 && (
          <Typography variant="body2" color="text.secondary">
            No longer applies:{" "}
            {preview.lostFields.map((f, i) => (
              <span key={f.key}>
                {i > 0 && ", "}
                <strong>{f.label}</strong>
              </span>
            ))}
            .
          </Typography>
        )}

        {!isSupportedTarget ? (
          <Typography variant="body2" color="text.secondary" fontStyle="italic">
            Transferring to {caseTypeTransferLabel(targetType)} isn&rsquo;t available yet — the
            entity-service doesn&rsquo;t accept it as a transfer target.
          </Typography>
        ) : targetType === "engagement" ? (
          <FormControl fullWidth size="small" required error={engagementTypeMissing}>
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
          <>
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
              Issue type isn&rsquo;t offered here — there&rsquo;s no way to update it on an
              existing case yet, only at creation. Not required to complete the transfer.
            </Typography>
          </>
        )}

        {preview.attachmentNeeded && (
          <Typography variant="body2" color="warning.main">
            {caseTypeTransferLabel(targetType)} requires at least one attachment, and this case
            currently has none. Add one from the Attachments tab before transferring.
          </Typography>
        )}

        <Typography variant="caption" color="text.secondary">
          SLA targets are type-specific in ServiceNow — this case&rsquo;s SLA clock will be
          recalculated against {caseTypeTransferLabel(targetType)}&rsquo;s policy once
          transferred, not carried over as-is.
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
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
