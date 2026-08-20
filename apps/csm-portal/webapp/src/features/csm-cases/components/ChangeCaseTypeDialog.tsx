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
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  FormHelperText,
  InputLabel,
  LinearProgress,
  MenuItem,
  Radio,
  RadioGroup,
  Select,
  Typography,
} from "@wso2/oxygen-ui";
import { Check, Upload } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useRef, useState, type ChangeEvent, type JSX } from "react";
import type { BeCaseType, BeEngagementType } from "@api/backend/types";
import type { Severity, SeverityOrUnset } from "@features/csm-dashboard/types/abtDashboard";
import { SEVERITY_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import { useSearchCatalogs } from "@features/csm-operations/api/useSearchCatalogs";
import { useCatalogItemVariables } from "@features/csm-operations/api/useCatalogItemVariables";
import CatalogVariableFields from "@features/csm-operations/components/CatalogVariableFields";
import {
  getFirstEmptyRequiredField,
  getUserEditableVariables,
  isAttachmentField,
} from "@features/csm-operations/utils/catalogVariables";
import QueryErrorState from "@components/QueryErrorState";
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

type Step = "pick" | "fields" | "review";
const STEP_NUMBER: Record<Step, number> = { pick: 1, fields: 2, review: 3 };

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
  /** Scopes the Service Request preview's catalog picker — same product the
   * case is already linked to. Absent for a case with no deployed product,
   * in which case the catalog picker shows its own "select a product first"
   * state, same as the create form. */
  deployedProductId?: string;
  /** Uploads a file to this case right away (the same handler the
   * Attachments tab uses) — offered on the Security Report Analysis field
   * step so the attachment requirement can actually be satisfied from here,
   * not just described. Omit to hide the uploader (e.g. no caseId yet). */
  onUploadAttachment?: (file: File) => void;
  isUploadingAttachment?: boolean;
  uploadAttachmentError?: string;
  isSubmitting: boolean;
  onClose: () => void;
  onSubmit: (submission: CaseTypeTransferSubmission) => void;
}

/**
 * Transfer a case between Case, Engagement, Security Report Analysis, and
 * Service Request (digiops-cs#2818). Three steps: pick the target type, fill
 * in whatever it needs, then review what's retained/lost and confirm. Only
 * Case <-> Engagement submits today — SRA/Service Request are still offered
 * on step 1, and their own field step is fully previewable (SRA's attachment
 * uploader really uploads; Service Request's catalog picker is real), so the
 * whole proposal is explorable — but the confirm button on step 3 stays
 * disabled for them until entity-service's `caseType` validator
 * (digiops-cs#2852) accepts them as targets.
 */
export default function ChangeCaseTypeDialog({
  currentType,
  currentSeverity,
  hasAttachments,
  deployedProductId,
  onUploadAttachment,
  isUploadingAttachment,
  uploadAttachmentError,
  isSubmitting,
  onClose,
  onSubmit,
}: ChangeCaseTypeDialogProps): JSX.Element {
  const targets = TRANSFERABLE_CASE_TYPES.filter((t) => t !== currentType);
  const [step, setStep] = useState<Step>("pick");
  const [targetType, setTargetType] = useState<BeCaseType>(targets[0]);
  const [engagementType, setEngagementType] = useState<BeEngagementType | "">("");
  const [severity, setSeverity] = useState<Severity | "">(
    currentSeverity === "unset" ? "" : currentSeverity,
  );
  const [catalogId, setCatalogId] = useState("");
  const [catalogItemId, setCatalogItemId] = useState("");
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const fileInputRef = useRef<HTMLInputElement>(null);

  const preview = computeTransferPreview(currentType, targetType, hasAttachments);
  const isSupportedTarget = SUPPORTED_TRANSFER_TARGETS.includes(targetType);
  const engagementTypeMissing = targetType === "engagement" && engagementType === "";
  const canSubmit = isSupportedTarget && !engagementTypeMissing && !isSubmitting;

  const catalogs = useSearchCatalogs(
    targetType === "service_request" ? deployedProductId : undefined,
  );
  const catalogItems = useMemo(
    () => catalogs.data?.find((c) => c.id === catalogId)?.catalogItems ?? [],
    [catalogs.data, catalogId],
  );
  const variables = useCatalogItemVariables(
    catalogId || undefined,
    catalogItemId || undefined,
  );
  const allVariables = useMemo(() => variables.data ?? [], [variables.data]);
  const renderableVars = useMemo(
    () => getUserEditableVariables(allVariables).filter((v) => !isAttachmentField(v)),
    [allVariables],
  );
  const firstEmptyRequired = useMemo(
    () => getFirstEmptyRequiredField(allVariables, answers),
    [allVariables, answers],
  );
  const noCatalogs =
    !!deployedProductId && catalogs.isSuccess && (catalogs.data?.length ?? 0) === 0;

  const handleTargetChange = (next: BeCaseType): void => {
    setTargetType(next);
    setEngagementType("");
    setCatalogId("");
    setCatalogItemId("");
    setAnswers({});
  };

  const pickFile = (): void => fileInputRef.current?.click();
  const onFileChange = (e: ChangeEvent<HTMLInputElement>): void => {
    const file = e.target.files?.[0];
    if (file) onUploadAttachment?.(file);
    // Reset so re-selecting the same file still fires onChange.
    if (fileInputRef.current) fileInputRef.current.value = "";
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
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        <Box sx={{ display: "flex", alignItems: "baseline", justifyContent: "space-between" }}>
          <span>Change case type</span>
          <Typography variant="caption" color="text.secondary">
            Step {STEP_NUMBER[step]} of 3
          </Typography>
        </Box>
      </DialogTitle>

      {step === "pick" && (
        <>
          <DialogContent dividers sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Typography variant="body2" color="text.secondary">
              Currently{" "}
              <Typography
                component="span"
                variant="body2"
                sx={{ fontWeight: 600, color: "text.primary" }}
              >
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
          </DialogContent>
          <DialogActions>
            <Button onClick={onClose}>Cancel</Button>
            <Button variant="contained" onClick={() => setStep("fields")}>
              Next
            </Button>
          </DialogActions>
        </>
      )}

      {step === "fields" && (
        <>
          <DialogContent dividers sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Typography variant="body2" color="text.secondary">
              What {caseTypeTransferLabel(targetType)} needs
            </Typography>

            {targetType === "engagement" && (
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
            )}

            {targetType === "case" && (
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

            {targetType === "security_report_analysis" &&
              (hasAttachments ? (
                <Typography
                  variant="body2"
                  sx={{ display: "flex", alignItems: "center", gap: 0.75, color: "success.main" }}
                >
                  <Check size={16} /> This case already has an attachment — nothing else needed.
                </Typography>
              ) : (
                <Box>
                  <Typography variant="body2" sx={{ mb: 1 }}>
                    Security Report Analysis requires at least one attachment, and this case
                    currently has none.
                  </Typography>
                  <input ref={fileInputRef} type="file" hidden onChange={onFileChange} aria-hidden />
                  <Button
                    size="small"
                    variant="outlined"
                    startIcon={<Upload size={14} />}
                    onClick={pickFile}
                    disabled={!onUploadAttachment || isUploadingAttachment}
                  >
                    {isUploadingAttachment ? "Uploading…" : "Upload attachment"}
                  </Button>
                  {isUploadingAttachment && <LinearProgress sx={{ mt: 1 }} />}
                  {uploadAttachmentError && (
                    <Typography variant="body2" color="error" sx={{ mt: 1 }}>
                      {uploadAttachmentError}
                    </Typography>
                  )}
                </Box>
              ))}

            {targetType === "service_request" && (
              <>
                <FormControl fullWidth size="small" required>
                  <InputLabel id="transfer-catalog-label">Catalog</InputLabel>
                  <Select
                    labelId="transfer-catalog-label"
                    label="Catalog"
                    value={catalogId}
                    onChange={(e) => {
                      setCatalogId(String(e.target.value));
                      setCatalogItemId("");
                      setAnswers({});
                    }}
                    disabled={!deployedProductId || catalogs.isLoading || noCatalogs}
                  >
                    {(catalogs.data ?? []).map((c) => (
                      <MenuItem key={c.id} value={c.id}>
                        {c.name ?? c.id}
                      </MenuItem>
                    ))}
                  </Select>
                  {!deployedProductId ? (
                    <FormHelperText>This case has no deployed product on file.</FormHelperText>
                  ) : catalogs.isError ? (
                    <FormHelperText error>Failed to load catalogs.</FormHelperText>
                  ) : catalogs.isLoading ? (
                    <FormHelperText>Loading catalogs…</FormHelperText>
                  ) : noCatalogs ? (
                    <FormHelperText>
                      No service catalogs are available for this product.
                    </FormHelperText>
                  ) : null}
                </FormControl>

                <FormControl fullWidth size="small" required>
                  <InputLabel id="transfer-catalog-item-label">Catalog item</InputLabel>
                  <Select
                    labelId="transfer-catalog-item-label"
                    label="Catalog item"
                    value={catalogItemId}
                    onChange={(e) => {
                      setCatalogItemId(String(e.target.value));
                      setAnswers({});
                    }}
                    disabled={!catalogId}
                  >
                    {catalogItems.map((ci) => (
                      <MenuItem key={ci.id} value={ci.id}>
                        {ci.name ?? ci.id}
                      </MenuItem>
                    ))}
                  </Select>
                  {!catalogId ? (
                    <FormHelperText>Select a catalog first</FormHelperText>
                  ) : catalogItems.length === 0 ? (
                    <FormHelperText>No items found in this catalog.</FormHelperText>
                  ) : null}
                </FormControl>

                {catalogItemId &&
                  (variables.isLoading ? (
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, py: 1 }}>
                      <CircularProgress size={18} />
                      <Typography variant="body2" color="text.secondary">
                        Loading request form…
                      </Typography>
                    </Box>
                  ) : variables.isError ? (
                    <QueryErrorState
                      message={
                        variables.error instanceof Error && variables.error.message.trim()
                          ? variables.error.message
                          : "Could not load the request form for this catalog item."
                      }
                      error={variables.error}
                    />
                  ) : renderableVars.length === 0 ? (
                    <Typography variant="body2" color="text.secondary">
                      This catalog item has no additional fields.
                    </Typography>
                  ) : (
                    <CatalogVariableFields
                      variables={renderableVars}
                      values={answers}
                      onChange={(id, value) => setAnswers((prev) => ({ ...prev, [id]: value }))}
                    />
                  ))}

                {catalogItemId && firstEmptyRequired && !variables.isLoading && (
                  <Typography variant="caption" color="text.secondary">
                    Required field: {firstEmptyRequired}
                  </Typography>
                )}
              </>
            )}
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setStep("pick")}>Back</Button>
            <Button variant="contained" onClick={() => setStep("review")}>
              Next
            </Button>
          </DialogActions>
        </>
      )}

      {step === "review" && (
        <>
          <DialogContent dividers sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Typography variant="body2" color="text.secondary">
              Transferring to{" "}
              <Typography
                component="span"
                variant="body2"
                sx={{ fontWeight: 600, color: "text.primary" }}
              >
                {caseTypeTransferLabel(targetType)}
              </Typography>
              .
            </Typography>

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

            {!isSupportedTarget && (
              <Typography variant="body2" color="text.secondary" fontStyle="italic">
                Transferring to {caseTypeTransferLabel(targetType)} isn&rsquo;t available yet —
                the entity-service doesn&rsquo;t accept it as a transfer target.
              </Typography>
            )}

            {preview.attachmentNeeded && (
              <Typography variant="body2" color="warning.main">
                {caseTypeTransferLabel(targetType)} requires at least one attachment, and this
                case still doesn&rsquo;t have one — go back to add one.
              </Typography>
            )}

            <Typography variant="caption" color="text.secondary">
              SLA targets are type-specific in ServiceNow — this case&rsquo;s SLA clock will be
              recalculated against {caseTypeTransferLabel(targetType)}&rsquo;s policy once
              transferred, not carried over as-is.
            </Typography>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setStep("fields")} disabled={isSubmitting}>
              Back
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
        </>
      )}
    </Dialog>
  );
}
