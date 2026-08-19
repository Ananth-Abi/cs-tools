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
import { CASE_DETAIL, SECURITY_CENTER } from "../utils/selectors";
import { expectSuccess } from "../utils/caseFlows";
import { caseSearchResponse } from "../utils/listSearch";

/** How long to allow for the list and its search results to resolve. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the Security Center's Security Report Analysis list
 * (`/projects/:projectId/security-center`).
 *
 * Exists so the create-report spec can check whether its report already exists
 * before raising another: reports are cases, and cases have no delete endpoint,
 * so an unguarded create accumulates permanent records on every run.
 */
export class SecurityCenterPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  searchInput(): Locator {
    return this.page.getByPlaceholder(SECURITY_CENTER.searchPlaceholder);
  }

  /**
   * Opens the Security Center and waits for the report list's search box.
   *
   * @param projectId - Project whose reports to list.
   */
  async open(projectId: string): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${SECURITY_CENTER.pathSegment}`,
    );
    await expect(this.searchInput()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Searches the report list and reports whether anything matched.
   *
   * The list's search covers case number, title and description, so passing a
   * report's **description** finds it regardless of its generated title — which
   * carries the creation date and so differs day to day.
   *
   * Waits for the search response produced by this very term, so a result is
   * never read mid-flight.
   *
   * @param searchText - Text to search for, typically the report's description.
   * @returns True when the search returns at least one report.
   */
  async hasReportMatching(searchText: string): Promise<boolean> {
    // The report list is backed by the same /cases/search endpoint as the cases
    // list, so it uses the same term-matched wait. `networkidle` would be unsafe
    // here for the same reason: a premature read reports "no report" and the
    // caller raises a duplicate that cannot be deleted.
    const searchResponse = caseSearchResponse(this.page, searchText);
    await this.searchInput().fill(searchText);
    const response = await searchResponse;

    await expectSuccess(response, "report search");

    // A result row always carries a case number. Checking for one is a positive
    // signal, rather than inferring a hit from the empty message being absent —
    // which would also be true mid-load.
    const rows = this.main().getByText(/^CS\d+$/);
    return (await rows.count()) > 0;
  }
}
