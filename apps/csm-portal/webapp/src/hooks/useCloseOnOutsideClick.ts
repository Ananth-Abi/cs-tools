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

import { useEffect, type RefObject } from "react";

/**
 * Closes a non-modal panel (e.g. a `hideBackdrop` Drawer used so the rest of
 * the page stays interactive while it's open) when the user clicks outside
 * it -- without fighting a toggle button's own click handler for state.
 *
 * A plain "close on any outside click" listener races a toggle button that
 * both opens a *different* item and (via the same click) would otherwise get
 * immediately closed again by this listener: whichever of the two state
 * updates lands second wins. Excluding clicks matched by `excludeSelector`
 * (e.g. every quick-preview eye button) sidesteps the race entirely -- a
 * click on one of those elements is left untouched here, so only that
 * button's own handler decides the next state (open a different item, or
 * toggle the current one closed).
 *
 * Listens on `mousedown` (capture phase, like MUI's own ClickAwayListener)
 * rather than `click`, so the close registers before whatever the click
 * target's own `click` handler does (e.g. a row navigating away).
 */
export function useCloseOnOutsideClick(
  active: boolean,
  contentRef: RefObject<HTMLElement | null>,
  excludeSelector: string,
  onClose: () => void,
): void {
  useEffect(() => {
    if (!active) return;

    function handlePointerDown(e: MouseEvent): void {
      const target = e.target;
      if (!(target instanceof Element)) return;
      if (contentRef.current?.contains(target)) return;
      if (target.closest(excludeSelector)) return;
      onClose();
    }

    document.addEventListener("mousedown", handlePointerDown, true);
    return () => document.removeEventListener("mousedown", handlePointerDown, true);
  }, [active, contentRef, excludeSelector, onClose]);
}
