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

import { describe, expect, it } from "vitest";
import { paginatedSelectMenuListProps, stripLightModeInlineStyles } from "@utils/common";
import { PAGINATED_SELECT_MENU_MAX_HEIGHT_PX } from "@constants/common";

describe("common utils", () => {
  it("paginatedSelectMenuListProps sets max height", () => {
    const props = paginatedSelectMenuListProps();
    expect(props.sx.maxHeight).toBe(PAGINATED_SELECT_MENU_MAX_HEIGHT_PX);
    expect(props.onScroll).toBeUndefined();
  });

  it("stripLightModeInlineStyles removes white backgrounds and dark text", () => {
    const html =
      '<p style="background-color: white; color: #000000">Hi</p>';
    const cleaned = stripLightModeInlineStyles(html);
    expect(cleaned).not.toContain("background-color: white");
    expect(cleaned).not.toContain("color: #000000");
  });

  it("stripLightModeInlineStyles removes light pastel hex backgrounds", () => {
    const html = '<div style="background-color: #bce4e8; padding: 0.01em 16px;">Note</div>';
    const cleaned = stripLightModeInlineStyles(html);
    expect(cleaned).not.toContain("#bce4e8");
    // Non-background declarations in the same style attribute are preserved.
    expect(cleaned).toContain("padding: 0.01em 16px");
  });

  it("stripLightModeInlineStyles removes near-white rgb() backgrounds", () => {
    const html = '<p style="background-color: rgb(245, 245, 245)">Hi</p>';
    const cleaned = stripLightModeInlineStyles(html);
    expect(cleaned).not.toContain("rgb(245, 245, 245)");
  });

  it("stripLightModeInlineStyles removes light 3-digit hex backgrounds", () => {
    const html = '<p style="background-color: #eee">Hi</p>';
    const cleaned = stripLightModeInlineStyles(html);
    expect(cleaned).not.toContain("#eee");
  });

  it("stripLightModeInlineStyles leaves dark/saturated backgrounds untouched", () => {
    const html = '<pre style="background-color: #1a1a1a; color: #f5f5f5">code</pre>';
    const cleaned = stripLightModeInlineStyles(html);
    expect(cleaned).toContain("background-color: #1a1a1a");
  });

  it("stripLightModeInlineStyles leaves a mid-tone (non-light) background untouched", () => {
    const html = '<div style="background-color: rgb(90, 90, 90)">Hi</div>';
    const cleaned = stripLightModeInlineStyles(html);
    expect(cleaned).toContain("rgb(90, 90, 90)");
  });

  it("stripLightModeInlineStyles removes a mid-gray background below WCAG AA contrast", () => {
    // #808080 (luminance ~0.216) contrasts with white text at ~3.95:1, below
    // the 4.5:1 AA minimum for normal text — a fixed luminance cutoff missed
    // this; the contrast-derived threshold must catch it.
    const html = '<div style="background-color: #808080">Hi</div>';
    const cleaned = stripLightModeInlineStyles(html);
    expect(cleaned).not.toContain("#808080");
  });

});
