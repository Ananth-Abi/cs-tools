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

import { Paper, Slide } from "@wso2/oxygen-ui";
import type { JSX, ReactNode } from "react";
import { createPortal } from "react-dom";

interface FloatingSlidePanelProps {
  /** Whether the panel is slid into view. Toggling this plays the same
   * slide transition a `Drawer` would (see below) -- it does not mount or
   * unmount this component itself; the caller decides what, if anything,
   * to render as `children` while closed. */
  open: boolean;
  /** Accessible name for the panel's `role="region"` landmark. Needed here
   * specifically because there is no backdrop or focus trap to otherwise
   * signal to assistive tech that this is a distinct panel rather than
   * part of the underlying page's own content. */
  ariaLabel: string;
  /** Matches `Drawer`'s own `PaperProps`/`slotProps.paper` width usage in
   * the two callers this replaced. */
  width?: number | string | Record<string, number | string>;
  children: ReactNode;
}

const DEFAULT_WIDTH = { xs: "100%", sm: 420 };

/**
 * A right-anchored floating panel that looks and animates like a MUI
 * `Drawer` (`anchor="right"`, default `variant="temporary"`) but isn't
 * built on `Modal` -- so it never enforces a focus trap or marks the rest
 * of the page `aria-hidden` while it's open, unlike `Drawer` (see the
 * CodeRabbit review discussion on cs-tools#1498: `pointerEvents: "none"`
 * plus `hideBackdrop` only ever stopped *pointer* clicks from being
 * swallowed by `Drawer`'s `Modal` base -- keyboard and screen-reader users
 * were still fully isolated from the rest of the page by `Modal`'s own
 * focus-trap and sibling `aria-hidden` behavior, which those props don't
 * touch).
 *
 * Built from `Slide` -- the same transition component `Drawer` uses
 * internally for its own slide animation, so the timing/easing match it
 * exactly by relying on the same theme defaults -- wrapping a plain
 * `position: fixed` `Paper`, neither of which carry any of `Modal`'s
 * accessibility-isolation behavior. Portalled to `document.body` (like
 * `RecentViewsButton`'s own floating panel) so its `position: fixed`
 * isn't at the mercy of some ancestor's stacking context (a CSS
 * `transform`/`filter`/`contain` anywhere between its mount point and the
 * page root would otherwise silently break the fixed positioning).
 *
 * `Slide` itself makes the panel `visibility: hidden` while fully closed --
 * neither focusable nor perceivable by a screen reader -- which is what
 * lets a caller keep this panel's content mounted (rather than
 * conditionally rendering the whole component) so the close transition can
 * play out instead of the panel vanishing mid-animation, without that
 * closed content being reachable in the meantime.
 */
export default function FloatingSlidePanel({
  open,
  ariaLabel,
  width = DEFAULT_WIDTH,
  children,
}: FloatingSlidePanelProps): JSX.Element {
  return createPortal(
    <Slide in={open} direction="left">
      <Paper
        role="region"
        aria-label={ariaLabel}
        elevation={16}
        square
        sx={{
          position: "fixed",
          top: 0,
          right: 0,
          bottom: 0,
          width,
          zIndex: (theme) => theme.zIndex.drawer,
          overflow: "hidden",
        }}
      >
        {children}
      </Paper>
    </Slide>,
    document.body,
  );
}
