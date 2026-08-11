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

import { Drawer } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import CasePreviewContent from "@features/csm-cases/components/CasePreviewContent";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";

interface CasePreviewDrawerProps {
  /** The row being previewed. `null` keeps the drawer mounted-but-closed, so
   * its close transition can play instead of unmounting mid-animation. */
  row: CsmCaseRow | null;
  onClose: () => void;
}

/**
 * Read-only "quick look" for a case row — subject/state/severity plus the
 * last handful of comments — without leaving the list. Deliberately thin:
 * this is a preview, not a second case detail page (no composer, no action
 * bar, no full activity/audit timeline). "View full details" always stays
 * one click away for anything this doesn't cover.
 *
 * The actual content lives in `CasePreviewContent` — extracted so
 * `TimeCardCasePreviewDrawer` can embed the same case-preview block beneath
 * a time card's own summary, rather than duplicating it.
 */
export default function CasePreviewDrawer({ row, onClose }: CasePreviewDrawerProps): JSX.Element {
  return (
    <Drawer
      anchor="right"
      open={!!row}
      onClose={onClose}
      slotProps={{ paper: { sx: { width: { xs: "100%", sm: 420 } } } }}
    >
      {row && <CasePreviewContent row={row} onClose={onClose} />}
    </Drawer>
  );
}
