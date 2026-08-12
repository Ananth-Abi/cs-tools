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
import { isSuccess } from "../utils/caseFlows";
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
    expect(
      isSuccess(response.status()),
      `case search failed: ${response.status()} ${await response.text()}`,
    ).toBe(true);

    const match = this.main().getByText(subject, { exact: true });
    return (await match.count()) > 0;
  }
}
