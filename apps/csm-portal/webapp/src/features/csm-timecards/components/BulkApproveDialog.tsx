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

import type { JSX } from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from "@wso2/oxygen-ui";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

interface BulkApproveDialogProps {
  /** The selected cards about to be approved — already filtered to ones the
   * signed-in approver is actually eligible to act on (see
   * `TimeCardsTable`'s `cardActions` check on the checkbox column). */
  cards: CsmTimeCard[];
  /** True while the bulk mutation is in flight. */
  isSubmitting: boolean;
  onClose: () => void;
  onConfirm: () => void;
}

/**
 * Confirms an "Approve N" bulk action before firing it — unlike
 * `TimeCardReviewDialog`, there's no comment field: approve alone doesn't
 * require one (see `useDecideCard`), and a single comment applied across N
 * different cards from N different engineers wouldn't mean anything specific
 * to any one of them. Purely a summary + confirm/cancel; the actual
 * per-card outcome (some cards can still fail — see `useBulkApproveCards`)
 * is reported by the caller after this closes, via the success/error
 * banners already used for a single decision.
 */
export default function BulkApproveDialog({
  cards,
  isSubmitting,
  onClose,
  onConfirm,
}: BulkApproveDialogProps): JSX.Element {
  const totalMinutes = cards.reduce((sum, c) => sum + c.totalMinutes, 0);

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        Approve {cards.length} time card{cards.length === 1 ? "" : "s"}?
      </DialogTitle>
      <DialogContent dividers>
        <Typography variant="body2" color="text.secondary">
          {cards.length} card{cards.length === 1 ? "" : "s"} · {totalMinutes} min total
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          color="primary"
          variant="outlined"
          onClick={onConfirm}
          disabled={isSubmitting || cards.length === 0}
        >
          Approve
        </Button>
      </DialogActions>
    </Dialog>
  );
}
