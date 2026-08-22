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

import { fireEvent, render, screen, within } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { navNodeById } from "@config/csmNavItems";
import { resetFeatureStatesForTests } from "@config/featureFlags";
import HelpPage from "./HelpPage";

function setOverrides(value: unknown): void {
  window.config = {
    ...window.config,
    CSM_PORTAL_FEATURE_OVERRIDES: value,
  } as Window["config"];
  resetFeatureStatesForTests();
}

beforeEach(() => {
  vi.spyOn(console, "warn").mockImplementation(() => undefined);
  setOverrides(undefined);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("HelpPage", () => {
  it("renders a table of contents with one anchor link per enabled topic, in nav order", () => {
    render(<HelpPage />);
    const nav = screen.getByRole("navigation", { name: "Help topics" });
    const help = navNodeById("help");
    const labels = (help?.children ?? []).map((child) => child.label);

    const links = within(nav).getAllByRole("link");
    expect(links.map((link) => link.textContent)).toEqual(labels);
  });

  it("points each TOC entry at the matching topic section's in-page anchor", () => {
    render(<HelpPage />);
    const link = screen.getByRole("link", { name: "Operations" });
    expect(link).toHaveAttribute("href", "#operations");
  });

  it("renders every topic's content below the table of contents, each in its own labelled section", () => {
    render(<HelpPage />);
    expect(screen.getByRole("heading", { name: "Overview" })).toBeVisible();
    expect(
      document.getElementById("operations")?.tagName.toLowerCase(),
    ).toBe("section");
  });

  it("drops a topic from both the table of contents and the rendered sections once it's disabled", () => {
    setOverrides({ "help.operations": "hidden" });
    render(<HelpPage />);
    expect(screen.queryByRole("link", { name: "Operations" })).toBeNull();
    expect(document.getElementById("operations")).toBeNull();
  });

  it("shows the back-to-top button only after scrolling past the top of the page", () => {
    render(<HelpPage />);
    expect(
      screen.queryByRole("button", { name: "Back to top" }),
    ).toBeNull();

    Object.defineProperty(window, "scrollY", { value: 400, configurable: true });
    fireEvent.scroll(window);

    expect(
      screen.getByRole("button", { name: "Back to top" }),
    ).toBeVisible();
  });

  it("scrolls back to the top of the page when the back-to-top button is clicked", () => {
    render(<HelpPage />);
    Object.defineProperty(window, "scrollY", { value: 400, configurable: true });
    fireEvent.scroll(window);

    const scrollTo = vi.fn();
    window.scrollTo = scrollTo;
    fireEvent.click(screen.getByRole("button", { name: "Back to top" }));

    expect(scrollTo).toHaveBeenCalledWith({ top: 0, behavior: "smooth" });
  });
});
