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
  DialogContentText,
  DialogTitle,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import { useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

function tabDisplayLabel(tab: CaseTabState): string {
  return tab.label ?? tab.caseId;
}

/**
 * Owns the close confirmation for a tab whose reply composer is open (a
 * best-effort "might have unsaved text" signal — see `CaseTabState.hasDraft`
 * and `useReportCaseTabDraft`'s own doc comment on the limits of that
 * signal). Split out from `CaseTabStrip` (a plain component) into its own
 * hook module so that file can stay a component-only export (fast-refresh
 * requires this).
 */
export function useCaseTabCloseConfirm(): {
  requestClose: (tab: CaseTabState) => void;
  dialog: JSX.Element;
} {
  const { closeTab } = useCaseTabsController();
  const [pending, setPending] = useState<CaseTabState | null>(null);

  const requestClose = (tab: CaseTabState): void => {
    if (tab.hasDraft) {
      setPending(tab);
      return;
    }
    closeTab(tab.id);
  };

  const dialog = (
    <Dialog open={pending !== null} onClose={() => setPending(null)} maxWidth="xs" fullWidth>
      <DialogTitle>Close this case tab?</DialogTitle>
      <DialogContent>
        <DialogContentText>
          {pending ? tabDisplayLabel(pending) : ""} has a reply in progress. Closing this
          tab will discard it.
        </DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={() => setPending(null)}>
          Keep tab open
        </Button>
        <Button
          variant="contained"
          color="error"
          onClick={() => {
            if (pending) closeTab(pending.id);
            setPending(null);
          }}
        >
          Close anyway
        </Button>
      </DialogActions>
    </Dialog>
  );

  return { requestClose, dialog };
}
