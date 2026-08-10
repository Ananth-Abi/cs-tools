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
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from "@wso2/oxygen-ui";
import { Check } from "@wso2/oxygen-ui-icons-react";
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
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
          {cards.length} card{cards.length === 1 ? "" : "s"} · {totalMinutes} min total
        </Typography>
        <Box
          sx={{
            maxHeight: 280,
            overflowY: "auto",
            border: 1,
            borderColor: "divider",
            borderRadius: 1,
          }}
        >
          {cards.map((c, i) => (
            <Box
              key={c.id}
              sx={{
                display: "flex",
                justifyContent: "space-between",
                gap: 1,
                px: 1.5,
                py: 1,
                borderBottom: i === cards.length - 1 ? 0 : 1,
                borderColor: "divider",
              }}
            >
              <Box sx={{ minWidth: 0 }}>
                <Typography variant="body2" noWrap>
                  {c.caseNumber}
                </Typography>
                <Typography variant="caption" color="text.secondary" noWrap display="block">
                  {c.userName}
                </Typography>
              </Box>
              <Typography variant="body2" color="text.secondary" sx={{ flexShrink: 0 }}>
                {c.totalMinutes} min
              </Typography>
            </Box>
          ))}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          color="success"
          variant="contained"
          startIcon={<Check size={16} />}
          onClick={onConfirm}
          disabled={isSubmitting || cards.length === 0}
        >
          Approve {cards.length}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
