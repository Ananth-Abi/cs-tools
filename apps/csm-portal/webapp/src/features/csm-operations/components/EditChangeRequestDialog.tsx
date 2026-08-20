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
  Alert,
  Box,
  Button,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  FormHelperText,
  Switch,
  TextField,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useSearchGroups } from "@api/useSearchGroups";
import type {
  BeChangeRequestDetail,
  BeGroup,
  BePatchChangeRequestPayload,
} from "@api/backend/types";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import {
  formatDateTimeLocal,
  isPastDateTime,
  parseDateTimeLocal,
} from "@utils/dateTime";
import { stripHtmlTagsPreservingLineBreaks } from "@utils/sanitizeHtml";

const { DateTimePicker, LocalizationProvider } = DatePickers;

interface EditChangeRequestDialogProps {
  cr: BeChangeRequestDetail;
  /** True while the PATCH is in flight; disables the actions. */
  isSaving: boolean;
  /**
   * User-facing message for the most recent failed save, if any. Rendered
   * inline in the dialog so the rejection is visible even if a page-level
   * error banner is occluded or the dialog is otherwise the only thing the
   * user is looking at.
   */
  saveError?: string | null;
  onClose: () => void;
  /** Submit only the changed fields (`PATCH /change-requests/{id}`). */
  onSave: (patch: BePatchChangeRequestPayload) => void;
}

/**
 * Convert a backend timestamp (`YYYY-MM-DD HH:MM:SS`, or ISO `T`-separated) to
 * the `YYYY-MM-DDTHH:MM` shape this form's state (and the DateTimePicker via
 * {@link parseDateTimeLocal}) uses. The value is treated as plain wall-clock
 * text so no timezone shift is applied.
 */
function toDateTimeLocal(raw?: string | null): string {
  const m = /^(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2})/.exec(raw?.trim() ?? "");
  return m ? `${m[1]}T${m[2]}` : "";
}

/** Convert a `datetime-local` value back to the BE's `YYYY-MM-DD HH:MM:SS`. */
function toBackendDateTime(local: string): string {
  return `${local.replace("T", " ")}:00`;
}

/**
 * The long-form plan fields come back as rich-text HTML but are edited here as
 * plain multiline text, so the stored markup is stripped for the initial
 * value. That is deliberately one-way: dirty-tracking compares against this
 * stripped value, so an untouched field is never part of the patch and the
 * stored markup survives. Editing one is an explicit rewrite of that field,
 * and plain text is a valid value for it.
 *
 * Block boundaries have to survive the strip. A stored multi-paragraph plan
 * flattened to one run-on line is a value the user never typed, and because
 * dirty-tracking is a string compare against it, the flattened text is what a
 * later edit would write back over the stored content.
 */
function toPlainTextValue(raw?: string | null): string {
  return raw ? stripHtmlTagsPreservingLineBreaks(raw) : "";
}

/**
 * Edit the change-request fields the BE allows updating: the planned window,
 * the assignment group, the customer approved / reviewed flags, and the
 * rollback and test plans. Only changed fields are sent, and the BE requires
 * at least one, so Save is disabled until something differs.
 */
export default function EditChangeRequestDialog({
  cr,
  isSaving,
  saveError,
  onClose,
  onSave,
}: EditChangeRequestDialogProps): JSX.Element {
  const initialPlannedStart = useMemo(
    () => toDateTimeLocal(cr.plannedStartOn),
    [cr.plannedStartOn],
  );
  const initialPlannedEnd = useMemo(
    () => toDateTimeLocal(cr.plannedEndOn),
    [cr.plannedEndOn],
  );
  const initialRollbackPlan = useMemo(
    () => toPlainTextValue(cr.rollbackPlan),
    [cr.rollbackPlan],
  );
  const initialTestPlan = useMemo(() => toPlainTextValue(cr.testPlan), [cr.testPlan]);
  const initialAssignedTeamId = cr.assignedTeam?.id ?? "";
  const [plannedStart, setPlannedStart] = useState(initialPlannedStart);
  const [plannedEnd, setPlannedEnd] = useState(initialPlannedEnd);
  const [approved, setApproved] = useState(!!cr.hasCustomerApproved);
  const [reviewed, setReviewed] = useState(!!cr.hasCustomerReviewed);
  const [assignedTeamId, setAssignedTeamId] = useState(initialAssignedTeamId);
  const [rollbackPlan, setRollbackPlan] = useState(initialRollbackPlan);
  const [testPlan, setTestPlan] = useState(initialTestPlan);

  // Client-side only, and only when both ends are set: the backing system
  // does its own validation and this must not become the thing that blocks a
  // legitimate save, so it surfaces inline rather than being enforced
  // server-side.
  const startDate = parseDateTimeLocal(plannedStart);
  const endDate = parseDateTimeLocal(plannedEnd);
  const plannedEndBeforeStart =
    !!startDate && !!endDate && endDate.getTime() <= startDate.getTime();

  const patch = useMemo<BePatchChangeRequestPayload>(() => {
    const next: BePatchChangeRequestPayload = {};
    if (plannedStart !== initialPlannedStart && plannedStart) {
      next.plannedStartOn = toBackendDateTime(plannedStart);
    }
    if (plannedEnd !== initialPlannedEnd && plannedEnd) {
      next.plannedEndOn = toBackendDateTime(plannedEnd);
    }
    if (approved !== !!cr.hasCustomerApproved) next.isCustomerApproved = approved;
    if (reviewed !== !!cr.hasCustomerReviewed) next.isCustomerReviewed = reviewed;
    if (assignedTeamId !== initialAssignedTeamId && assignedTeamId) {
      next.assignedTeamId = assignedTeamId;
    }
    // Unlike the pickers above, an emptied plan field is a real edit the BE
    // can accept, so "" is sent rather than skipped.
    if (rollbackPlan !== initialRollbackPlan) next.rollbackPlan = rollbackPlan;
    if (testPlan !== initialTestPlan) next.testPlan = testPlan;
    return next;
  }, [
    plannedStart,
    initialPlannedStart,
    plannedEnd,
    initialPlannedEnd,
    approved,
    reviewed,
    assignedTeamId,
    initialAssignedTeamId,
    rollbackPlan,
    initialRollbackPlan,
    testPlan,
    initialTestPlan,
    cr.hasCustomerApproved,
    cr.hasCustomerReviewed,
  ]);

  const hasChanges = Object.keys(patch).length > 0;
  // Non-blocking: editing a CR's planned start to a past instant is unusual
  // but not forbidden (e.g. recording when it actually started), so this
  // only warns.
  const plannedStartIsPast = isPastDateTime(startDate);

  // The backend rejects a patch containing both isCustomerApproved and
  // isCustomerReviewed outright — they, and requestApproval, are mutually
  // exclusive. Mirror that here: once one of the two has been changed away
  // from its saved value, lock the other to its current value until this
  // save goes through (or the first change is undone), so the dialog can
  // never build the two-key payload the backend always refuses.
  const approvedChanged = approved !== !!cr.hasCustomerApproved;
  const reviewedChanged = reviewed !== !!cr.hasCustomerReviewed;
  const approvedLocked = reviewedChanged;
  const reviewedLocked = approvedChanged;

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Edit change request</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {saveError && (
            <Alert severity="error" sx={{ width: "100%" }}>
              {saveError}
            </Alert>
          )}
          {/*
            No `clearable` on either picker. The patch payload has no way to
            express "remove the planned date" — `plannedStartOn`/`plannedEndOn`
            are `string | undefined`, and an omitted key means "leave it
            alone" — so a clear affordance would appear to work and then
            silently save nothing. Widening the payload to express a null
            clear is the fix if this is ever actually needed.
          */}
          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DateTimePicker
              label="Planned start"
              value={startDate}
              onChange={(next) =>
                setPlannedStart(
                  next instanceof Date && !Number.isNaN(next.getTime())
                    ? formatDateTimeLocal(next)
                    : "",
                )
              }
              slotProps={{
                textField: {
                  size: "small",
                  fullWidth: true,
                  helperText: plannedStartIsPast
                    ? "This date is in the past."
                    : undefined,
                },
              }}
            />
            <DateTimePicker
              label="Planned end"
              value={endDate}
              onChange={(next) =>
                setPlannedEnd(
                  next instanceof Date && !Number.isNaN(next.getTime())
                    ? formatDateTimeLocal(next)
                    : "",
                )
              }
              slotProps={{
                textField: {
                  size: "small",
                  fullWidth: true,
                  error: plannedEndBeforeStart,
                  helperText: plannedEndBeforeStart
                    ? "Planned end must be after planned start."
                    : undefined,
                },
              }}
            />
          </LocalizationProvider>
          <Box>
            <FormControlLabel
              control={
                <Switch
                  checked={approved}
                  disabled={isSaving || approvedLocked}
                  // Guard the handler too, not just the `disabled` attribute:
                  // it's the actual mutual-exclusion enforcement, so it must
                  // not depend on the DOM ignoring input while disabled.
                  onChange={(e) => {
                    if (approvedLocked) return;
                    setApproved(e.target.checked);
                  }}
                />
              }
              label="Customer approved"
            />
            {approvedLocked && (
              <FormHelperText>
                Save the customer-reviewed change first — approved and
                reviewed can&apos;t be changed in the same save.
              </FormHelperText>
            )}
          </Box>
          <Box>
            <FormControlLabel
              control={
                <Switch
                  checked={reviewed}
                  disabled={isSaving || reviewedLocked}
                  onChange={(e) => {
                    if (reviewedLocked) return;
                    setReviewed(e.target.checked);
                  }}
                />
              }
              label="Customer reviewed"
            />
            {reviewedLocked && (
              <FormHelperText>
                Save the customer-approved change first — approved and
                reviewed can&apos;t be changed in the same save.
              </FormHelperText>
            )}
          </Box>
          <AsyncEntitySelect<BeGroup>
            id="cr-edit-assigned-team"
            label="Assignment group"
            placeholder="Search groups…"
            value={assignedTeamId}
            onChange={setAssignedTeamId}
            disabled={isSaving}
            useSearch={useSearchGroups}
            getId={(g) => g.id}
            getLabel={(g) => g.name}
            knownLabel={cr.assignedTeam?.name}
            helperText="Required before approval can be requested."
          />
          <TextField
            label="Rollback plan"
            value={rollbackPlan}
            onChange={(e) => setRollbackPlan(e.target.value)}
            disabled={isSaving}
            multiline
            minRows={3}
            fullWidth
            size="small"
            helperText="How this change is backed out if it goes wrong."
          />
          <TextField
            label="Test plan"
            value={testPlan}
            onChange={(e) => setTestPlan(e.target.value)}
            disabled={isSaving}
            multiline
            minRows={3}
            fullWidth
            size="small"
            helperText="How the change is verified once implemented."
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => onSave(patch)}
          disabled={isSaving || !hasChanges || plannedEndBeforeStart}
        >
          {isSaving ? "Saving…" : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
