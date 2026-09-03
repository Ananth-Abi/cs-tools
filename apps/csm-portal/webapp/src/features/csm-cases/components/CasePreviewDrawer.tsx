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

import { Box } from "@wso2/oxygen-ui";
import { useRef, type JSX } from "react";
import FloatingSlidePanel from "@components/FloatingSlidePanel";
import CasePreviewContent from "@features/csm-cases/components/CasePreviewContent";
import { QUICK_PREVIEW_EYE_SELECTOR } from "@features/csm-cases/utils/quickPreviewEye";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";
import { useCloseOnOutsideClick } from "@hooks/useCloseOnOutsideClick";

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
  const contentRef = useRef<HTMLDivElement | null>(null);
  useCloseOnOutsideClick(!!row, contentRef, QUICK_PREVIEW_EYE_SELECTOR, onClose);

  return (
    // `FloatingSlidePanel` (not `Drawer`) -- a `Drawer` is `Modal`-backed,
    // which enforces a focus trap and marks the rest of the page
    // `aria-hidden` for as long as it's open, regardless of `hideBackdrop`
    // or pointer-events tricks (those only ever affected mouse clicks, not
    // `Modal`'s own accessibility isolation). This panel has no backdrop
    // and no modal behavior at all, so the rest of the page stays fully
    // interactive for every input method -- not just the mouse -- letting
    // a click on a different row's quick-preview eye land normally while
    // this preview is already open. `useCloseOnOutsideClick` below replaces
    // the click-to-close behavior a `Drawer`'s backdrop would otherwise
    // give, minus the eye buttons (their own onClick already decides the
    // next state -- open a different row, or toggle the current one
    // closed).
    <FloatingSlidePanel open={!!row} ariaLabel="Case preview">
      {row && (
        <Box ref={contentRef} sx={{ height: "100%" }}>
          <CasePreviewContent row={row} onClose={onClose} />
        </Box>
      )}
    </FloatingSlidePanel>
  );
}
