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

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";
import FloatingSlidePanel from "@components/FloatingSlidePanel";

describe("FloatingSlidePanel", () => {
  it("renders its children and the region landmark when open", () => {
    render(
      <FloatingSlidePanel open ariaLabel="Test panel">
        <button type="button">Inside content</button>
      </FloatingSlidePanel>,
    );

    const region = screen.getByRole("region", { name: "Test panel" });
    expect(region).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Inside content" })).toBeInTheDocument();
  });

  it("portals to document.body rather than rendering inline in its parent", () => {
    const { container } = render(
      <div data-testid="parent-wrapper">
        <FloatingSlidePanel open ariaLabel="Test panel">
          <span>Panel content</span>
        </FloatingSlidePanel>
      </div>,
    );

    // The panel's own content shouldn't be a descendant of the component's
    // React-tree parent -- it should be a portalled sibling under body.
    expect(container.querySelector('[role="region"]')).not.toBeInTheDocument();
    expect(document.body.querySelector('[role="region"]')).toBeInTheDocument();
  });

  it("does not add any modal isolation to the rest of the page (no backdrop, no aria-hidden siblings)", () => {
    render(
      <>
        <button type="button">Sibling button</button>
        <FloatingSlidePanel open ariaLabel="Test panel">
          <span>Panel content</span>
        </FloatingSlidePanel>
      </>,
    );

    // A Modal-backed Drawer would mark this sibling (or an ancestor of it)
    // `aria-hidden="true"` while open -- this panel must not.
    const sibling = screen.getByRole("button", { name: "Sibling button" });
    expect(sibling.closest('[aria-hidden="true"]')).not.toBeInTheDocument();
    expect(document.body).not.toHaveAttribute("aria-hidden");

    // No backdrop element intercepting clicks on the rest of the page.
    expect(document.querySelector(".MuiBackdrop-root")).not.toBeInTheDocument();

    sibling.focus();
    expect(sibling).toHaveFocus();
  });

  it("keeps the panel out of the accessibility tree while closed, without unmounting its children", () => {
    render(
      <FloatingSlidePanel open={false} ariaLabel="Test panel">
        <button type="button">Inside content</button>
      </FloatingSlidePanel>,
    );

    // `Slide` marks a fully-closed panel `visibility: hidden` -- not
    // perceivable and not focusable -- rather than removing it from the DOM,
    // so a subsequent open can play its enter transition from a known state.
    expect(screen.queryByRole("region", { name: "Test panel" })).not.toBeInTheDocument();
    const hiddenRegion = document.body.querySelector('[role="region"]');
    expect(hiddenRegion).toBeInTheDocument();
    expect(hiddenRegion).toHaveStyle({ visibility: "hidden" });
  });

  it("stacks below modal dialogs (theme.zIndex.drawer, not theme.zIndex.modal + 1) so a dialog opened while the panel is exiting stays on top", () => {
    render(
      <FloatingSlidePanel open ariaLabel="Test panel">
        <span>Panel content</span>
      </FloatingSlidePanel>,
    );

    const region = document.body.querySelector('[role="region"]');
    expect(region).toBeInTheDocument();
    const panelZIndex = Number(window.getComputedStyle(region as Element).zIndex);
    // MUI's default scale: drawer (1200) < modal (1300). The panel must sit
    // at drawer level so TimeCardReviewDialog (a Modal-backed Dialog) can
    // still cover it if opened while the panel is mid exit-transition.
    expect(panelZIndex).toBe(1200);
    expect(panelZIndex).toBeLessThan(1300);
  });
});
