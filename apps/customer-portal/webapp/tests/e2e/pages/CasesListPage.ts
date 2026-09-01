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
import { CASES_LIST, CASE_DETAIL } from "../utils/selectors";
import { expectSuccess } from "../utils/caseFlows";
import { caseSearchResponse } from "../utils/listSearch";

/** How long to allow for the list and its search results to resolve. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for a project's cases list
 * (`/projects/:projectId/support/cases`).
 */
export class CasesListPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  searchInput(): Locator {
    return this.page.getByPlaceholder(CASES_LIST.searchPlaceholder);
  }

  /**
   * Opens the list and waits for the search box.
   *
   * @param projectId - Project whose cases to list.
   */
  async open(projectId: string): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${CASES_LIST.pathSegment}`,
    );
    await expect(this.searchInput()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Searches the list and reports whether a case with this exact subject is
   * listed.
   *
   * The search matches id, title or description, so it can return near misses —
   * "subscription case S1" also matches "subscription case S10". The exact-text
   * check is what makes this a reliable existence test rather than a fuzzy one.
   *
   * @param subject - Exact case subject to look for.
   * @returns True when a row with that subject is present.
   */
  async hasCaseWithSubject(subject: string): Promise<boolean> {
    // Wait for the search request carrying THIS subject — see listSearch for why
    // `networkidle` is not a safe signal here.
    const searchResponse = caseSearchResponse(this.page, subject);

    await this.searchInput().fill(subject);
    const response = await searchResponse;

    // A failed search returns no rows, which is indistinguishable from "no such
    // case" — so assert it succeeded rather than silently treating it as absent.
    await expectSuccess(response, "case search");

    const match = this.main().getByText(subject, { exact: true });
    return (await match.count()) > 0;
  }

  /** The list's heading, which names which list is on screen. */
  heading(name: string): Locator {
    return this.main().getByText(name, { exact: true });
  }

  /**
   * A filter's label inside the panel.
   *
   * Returns every match rather than one: MUI renders a Select's label twice —
   * once as the floating InputLabel and once inside the outline's legend — so a
   * single-element locator would be a strict-mode violation. Callers assert on
   * the count.
   *
   * @param label - Filter label as rendered.
   * @returns Locator for its label nodes.
   */
  filterLabel(label: string): Locator {
    return this.main().getByText(label, { exact: true });
  }

  /** The Created By filter's label. Withheld on My Cases. */
  createdByFilter(): Locator {
    return this.filterLabel(CASES_LIST.createdByFilterLabel);
  }

  /**
   * Opens the filter panel.
   *
   * The panel is collapsed on load and its contents are unmounted, not hidden —
   * so before this runs, *no* filter is in the DOM and "filter X is absent"
   * holds trivially. Waits for a filter every list offers, which is what makes
   * an absence assertion afterwards meaningful.
   */
  async openFilters(): Promise<void> {
    await this.main()
      .getByRole("button", { name: CASES_LIST.filtersButton, exact: true })
      .click();
    await expect(
      this.filterLabel(CASES_LIST.severityFilterLabel).first(),
      "the filter panel should be open",
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * The case rows.
   *
   * ListCard sets `role="button"` on each row (it is clickable), so the rows are
   * buttons — but so are the page's controls, hence the narrowing below by the
   * per-row "Created by" text is done in `rowCreators()` rather than here.
   */
  rows(): Locator {
    return this.main()
      .getByRole("button")
      .filter({ hasText: CASES_LIST.createdByPrefix });
  }

  /**
   * The creator named on each row, in list order.
   *
   * ListCard renders "Created by <x>" only when the case carries a creator, so
   * rows without one are simply absent from the result rather than empty
   * strings — callers compare the distinct values, which an empty entry would
   * distort.
   *
   * @returns One creator per row that names one.
   */
  async rowCreators(): Promise<string[]> {
    const texts = await this.main()
      .getByText(new RegExp(`^${CASES_LIST.createdByPrefix}\\S`))
      .allInnerTexts();
    return texts.map((text) =>
      text.replace(CASES_LIST.createdByPrefix, "").trim(),
    );
  }

  /**
   * Total number of cases the list reports, from the "Showing X of Y cases" bar.
   *
   * The total — not the shown count — is the filtered size of the whole result
   * set, so it is what a comparison between two lists has to use; the shown
   * count is capped by the page size.
   *
   * @returns The total, or null when the bar is not rendered.
   */
  async totalCount(): Promise<number | null> {
    const bar = this.main().getByText(CASES_LIST.resultsCountPattern).first();
    await expect(bar).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    const match = CASES_LIST.resultsCountPattern.exec(await bar.innerText());
    return match ? Number(match[2]) : null;
  }
}
