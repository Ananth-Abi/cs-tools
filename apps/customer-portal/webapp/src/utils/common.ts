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

import type { UIEvent } from "react";
import DOMPurify from "dompurify";
import { PAGINATED_SELECT_MENU_MAX_HEIGHT_PX } from "@constants/common";

// Harden all sanitized <a target="_blank"> links against reverse tabnabbing.
// Registered once at module load; applies to every DOMPurify.sanitize() call in the app.
if (typeof window !== "undefined") {
  DOMPurify.addHook("afterSanitizeAttributes", (node) => {
    if (node.tagName === "A" && node.getAttribute("target") === "_blank") {
      node.setAttribute("rel", "noopener noreferrer");
    }
  });
}

/**
 * MenuList props for paginated selects: fixed max height + optional scroll handler.
 *
 * @param {((e: UIEvent<HTMLElement>) => void) | undefined} onScroll - Near-bottom handler for fetchNextPage.
 * @returns {object} MUI MenuProps.MenuListProps fragment.
 */
export function paginatedSelectMenuListProps(
  onScroll?: (e: UIEvent<HTMLElement>) => void,
): {
  onScroll?: (e: UIEvent<HTMLElement>) => void;
  sx: { maxHeight: number; overflowY: "auto" };
} {
  return {
    ...(onScroll ? { onScroll } : {}),
    sx: {
      maxHeight: PAGINATED_SELECT_MENU_MAX_HEIGHT_PX,
      overflowY: "auto",
    },
  };
}

/**
 * Strips light/pastel inline background declarations from style attributes so
 * dark-mode containers no longer render light boxes (which read poorly against
 * the app's light dark-mode text) on a dark background. Lightness is judged by
 * relative luminance rather than a fixed near-white check, so pastel colors
 * (e.g. a light teal `#bce4e8`) are caught too, not just near-white ones.
 * Everything else (code-block backgrounds, borders, shadows, text colors) is
 * intentionally left untouched so light-mode and structural styling stay intact.
 *
 * @param html - Raw HTML string.
 * @returns HTML with light background declarations removed.
 */
export function stripLightModeInlineStyles(html: string): string {
  return html.replace(
    /style\s*=\s*"([^"]*)"/gi,
    (_match, styleContent: string) => {
      const declarations = styleContent.split(";");
      const filtered = declarations.filter((decl) => {
        const normalized = decl.toLowerCase().replace(/\s+/g, " ").trim();
        if (!normalized) return false;
        if (/^background(-color)?\s*:/.test(normalized) && isLightBackground(normalized))
          return false;
        if (/^color\s*:/.test(normalized) && isDarkColor(normalized))
          return false;
        return true;
      });
      const cleaned = filtered.join(";").replace(/;+$/, "").trim();
      if (!cleaned) return "";
      return `style="${cleaned}"`;
    },
  );
}

/** DOMPurify config for backend description/body HTML: strips tables and code blocks. */
export const DESCRIPTION_PURIFY_CONFIG = {
  FORBID_TAGS: ["table", "thead", "tbody", "tfoot", "tr", "th", "td", "colgroup", "col", "code", "pre"],
  FORBID_CONTENTS: ["table", "thead", "tbody", "tfoot", "tr", "th", "td", "colgroup", "col", "code", "pre"],
};

// WCAG AA minimum contrast ratio for normal-size text.
const MIN_CONTRAST_RATIO = 4.5;
// Dark-mode default text renders effectively white; used only to derive the
// background threshold below, not to special-case any particular text color.
const DARK_MODE_TEXT_LUMINANCE = 1;

// A background is stripped once its own contrast against dark-mode text would
// drop below MIN_CONTRAST_RATIO — i.e. WCAG contrast = (L_text + 0.05) /
// (L_bg + 0.05) solved for the L_bg at which that ratio equals the minimum.
// Deriving it this way (rather than an eyeballed constant) means a background
// like #808080 (luminance ~0.22, ~3.95:1 against white — below AA) is caught;
// a fixed 0.4 threshold missed it. Pure/near-white sits at ~1.0; a pastel like
// `#bce4e8` sits at ~0.72 — both clear this derived threshold too.
const LIGHT_BACKGROUND_LUMINANCE_THRESHOLD =
  (DARK_MODE_TEXT_LUMINANCE + 0.05) / MIN_CONTRAST_RATIO - 0.05;

/** WCAG-style relative luminance (0-1) for an sRGB triplet (0-255 channels). */
function relativeLuminance(r: number, g: number, b: number): number {
  const [rl, gl, bl] = [r, g, b].map((channel) => {
    const s = channel / 255;
    return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  });
  return 0.2126 * rl + 0.7152 * gl + 0.0722 * bl;
}

function isLightBackground(bgDecl: string): boolean {
  // Explicit named light background already known to read poorly in dark mode.
  if (/^background(-color)?\s*:\s*white\s*$/.test(bgDecl)) return true;

  const rgbMatch = bgDecl.match(
    /^background(?:-color)?\s*:\s*rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)\s*$/,
  );
  if (rgbMatch) {
    const [, r, g, b] = rgbMatch.map(Number);
    return relativeLuminance(r, g, b) > LIGHT_BACKGROUND_LUMINANCE_THRESHOLD;
  }

  const hex3 = bgDecl.match(/^background(?:-color)?\s*:\s*#([0-9a-f]{3})\s*$/);
  if (hex3) {
    const [rv, gv, bv] = hex3[1].split("").map((c) => parseInt(c + c, 16));
    return relativeLuminance(rv, gv, bv) > LIGHT_BACKGROUND_LUMINANCE_THRESHOLD;
  }

  const hex6 = bgDecl.match(/^background(?:-color)?\s*:\s*#([0-9a-f]{6})\s*$/);
  if (hex6) {
    const rv = parseInt(hex6[1].slice(0, 2), 16);
    const gv = parseInt(hex6[1].slice(2, 4), 16);
    const bv = parseInt(hex6[1].slice(4, 6), 16);
    return relativeLuminance(rv, gv, bv) > LIGHT_BACKGROUND_LUMINANCE_THRESHOLD;
  }

  return false;
}

function isDarkColor(colorDecl: string): boolean {
  // Named dark colors
  if (/^color\s*:\s*(black|#000(000)?|#1[0-9a-f]{5}|#2[0-9a-f]{5})\s*$/.test(colorDecl))
    return true;
  // rgb(r, g, b) where all channels are below 100 (dark)
  const rgbMatch = colorDecl.match(/^color\s*:\s*rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)\s*$/);
  if (rgbMatch) {
    const [, r, g, b] = rgbMatch.map(Number);
    return r < 100 && g < 100 && b < 100;
  }
  // 3-digit or 6-digit hex colors that are dark (luminance heuristic)
  const hex3 = colorDecl.match(/^color\s*:\s*#([0-9a-f]{3})\s*$/);
  if (hex3) {
    const [rv, gv, bv] = hex3[1].split("").map((c) => parseInt(c + c, 16));
    return rv < 100 && gv < 100 && bv < 100;
  }
  const hex6 = colorDecl.match(/^color\s*:\s*#([0-9a-f]{6})\s*$/);
  if (hex6) {
    const rv = parseInt(hex6[1].slice(0, 2), 16);
    const gv = parseInt(hex6[1].slice(2, 4), 16);
    const bv = parseInt(hex6[1].slice(4, 6), 16);
    return rv < 100 && gv < 100 && bv < 100;
  }
  return false;
}
