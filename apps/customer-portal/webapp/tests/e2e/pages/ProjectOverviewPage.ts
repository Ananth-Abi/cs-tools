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

import { type Locator, type Page, expect } from "../fixtures/test";
import {
  CASE_DETAIL,
  PROJECT_DETAILS,
  PROJECT_OVERVIEW,
} from "../utils/selectors";

/** How long to allow for the dashboard and the overview cards to load — each is
 * skeletonised while its queries resolve, well beyond the 5s default. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the Overview tab of the project details page
 * (`/projects/:projectId/project-details`).
 *
 * The cards render each field as a label `<Typography>` followed by a sibling
 * value `<Typography>`, with no ids or test ids, so a field's value is reached
 * by scoping to the smallest container holding its label. `fieldValue()` does
 * that; everything else here addresses labels and section headings directly.
 */
export class ProjectOverviewPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * Navigates from the project dashboard to the Overview tab via the side nav.
   *
   * Overview is the default tab, but it is selected explicitly so the test does
   * not depend on that default.
   *
   * @param projectId - Project whose overview to open.
   */
  async openOverviewTab(projectId: string): Promise<void> {
    await this.page.goto(`/projects/${projectId}/dashboard`);

    const navItem = this.page.getByRole("button", {
      name: PROJECT_DETAILS.navItem,
    });
    await expect(navItem).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await navItem.click();
    await expect(this.page).toHaveURL(
      new RegExp(`/projects/${projectId}/${PROJECT_DETAILS.pathSegment}`),
    );

    await this.overviewTab().click();
    // The Project Information heading renders before its data does, so wait on a
    // field label instead — by then the card has resolved.
    await expect(
      this.label(
        PROJECT_OVERVIEW.sections.projectInformation,
        PROJECT_OVERVIEW.labels.createdDate,
        PROJECT_OVERVIEW.labels.projectName,
      ),
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  overviewTab(): Locator {
    return this.page.getByRole("tab", {
      name: new RegExp(PROJECT_DETAILS.tabs.overview),
    });
  }

  /** A section heading, e.g. "Project Information". */
  sectionHeading(name: string): Locator {
    return this.main().getByRole("heading", { name, exact: true });
  }

  /**
   * The card containing a given section heading.
   *
   * Labels repeat across sections — "Remaining" appears both under Subscription
   * Period and in Service Hours Allocations — so a page-wide text match is
   * ambiguous. Every field assertion is scoped through here instead.
   *
   * Two details this depends on, both established by probing the live page:
   *
   * - The `has:` locators must be page-level, not `main`-scoped. Playwright
   *   resolves them relative to each candidate element, so a `main`-scoped
   *   locator would look for <main> *inside* the card and match nothing.
   * - Filtering on the heading alone is not enough: the innermost div holding it
   *   is the header wrapper, which contains none of the fields. Requiring an
   *   `anchorLabel` from the section's body narrows it to a div holding both, so
   *   `.last()` lands on the card rather than on a header or an ancestor.
   *
   * @param headingName - The section's heading text.
   * @param anchorLabel - A label known to sit in that section's body.
   * @returns Locator for that section's card.
   */
  section(headingName: string, anchorLabel: string): Locator {
    return this.main()
      .locator("div")
      .filter({
        has: this.page.getByRole("heading", { name: headingName, exact: true }),
      })
      .filter({ has: this.page.getByText(anchorLabel, { exact: true }) })
      .last();
  }

  /**
   * A field label within a section.
   *
   * @param sectionHeading - Heading of the section to search inside.
   * @param anchorLabel - A label known to sit in that section's body.
   * @param name - Exact label text.
   * @returns Locator for the label.
   */
  label(sectionHeading: string, anchorLabel: string, name: string): Locator {
    return this.section(sectionHeading, anchorLabel).getByText(name, {
      exact: true,
    });
  }

  /** The name shown under "Project Name". */
  projectName(name: string): Locator {
    return this.main().getByText(name, { exact: true });
  }

  /** The project key chip beside the name. */
  projectKeyChip(key: string): Locator {
    return this.main().getByText(key, { exact: true });
  }

  /**
   * Placeholders (`"--"`) within a section — rendered where the API returned no
   * value. Asserting a section has none is how a spec distinguishes "the card
   * rendered" from "the card rendered with data".
   *
   * Section-scoped rather than page-wide because whether a field is populated is
   * per-field project data: on the current fixture Support Tier and Go Live Date
   * are legitimately empty, so a page-wide count of zero would fail on a data
   * condition rather than on a defect.
   *
   * @param sectionHeading - Heading of the section to search inside.
   * @param anchorLabel - A label known to sit in that section's body.
   * @returns Locator for the placeholders in that section.
   */
  emptyValuePlaceholders(
    sectionHeading: string,
    anchorLabel: string,
  ): Locator {
    return this.section(sectionHeading, anchorLabel).getByText(
      PROJECT_OVERVIEW.emptyValue,
      { exact: true },
    );
  }
}
