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

import { type Locator, type Page, expect } from "@playwright/test";
import { CASE_DETAIL } from "../utils/selectors";
import { isSuccess } from "../utils/caseFlows";

/**
 * Page object for the case detail page
 * (`/projects/:projectId/support/cases/:caseId`).
 *
 * Only the state-change actions are modelled here so far. Which buttons the
 * action row renders depends on the case's current status — see
 * `getAvailableCaseActions` in src/features/support/utils/support.ts.
 */
export class CaseDetailPage {
  constructor(private readonly page: Page) {}

  /** The app's <main> region. Everything on this page is scoped to it so the
   * surrounding chrome — notably a promo banner with its own "Close" dismiss
   * control — cannot make a locator ambiguous. */
  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /** The "Close" action button. Present while the case is open; once closed the
   * action row swaps it for "Open Related Case". */
  closeButton(): Locator {
    return this.main().getByRole("button", {
      name: CASE_DETAIL.closeButton,
      exact: true,
    });
  }

  confirmDialog(): Locator {
    return this.page.getByRole("dialog");
  }

  confirmButton(): Locator {
    return this.confirmDialog().getByRole("button", {
      name: CASE_DETAIL.confirmDialog.confirmButton,
      exact: true,
    });
  }

  /** Any element rendering the closed-status text — the header chip. */
  closedStatusChip(): Locator {
    return this.main()
      .getByText(CASE_DETAIL.closedStatus, { exact: true })
      .first();
  }

  /**
   * Clicks "Close" and confirms the dialog, then waits for the PATCH to
   * succeed.
   *
   * The button stays disabled until the case-states metadata resolves (the
   * action needs a state key to patch with), so this waits for it to be
   * enabled rather than clicking into a no-op.
   *
   * @returns The successful PATCH response for the caller to assert on.
   */
  async closeCase(): Promise<import("@playwright/test").Response> {
    await expect(this.closeButton()).toBeEnabled();
    await this.closeButton().click();

    // Closing is confirmation-gated; the dialog must appear before confirming.
    await expect(this.confirmDialog()).toBeVisible();
    await expect(
      this.page.getByText(CASE_DETAIL.confirmDialog.title),
    ).toBeVisible();

    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          r.url().includes("/cases/") &&
          r.request().method() === "PATCH" &&
          isSuccess(r.status()),
      ),
      this.confirmButton().click(),
    ]);
    return response;
  }
}
