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

import {
  type Locator,
  type Page,
  type Response,
  expect,
} from "../fixtures/test";
import {
  CASE_COMMENT_INPUT,
  CASE_DETAIL,
  CASE_DETAILS_PANEL,
} from "../utils/selectors";
import { isSuccess } from "../utils/caseFlows";

/** How long to allow for a case's detail page to resolve — the header is
 * skeletonised while the case loads, well beyond the 5s default. */
const LOAD_TIMEOUT_MS = 60_000;

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

  /**
   * Opens a case's detail page directly.
   *
   * @param projectId - Project the case belongs to.
   * @param caseId - Case sysid.
   */
  async open(projectId: string, caseId: string): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${CASE_DETAIL.pathSegment}/${caseId}`,
    );
    // The header renders once the case resolves; the case number is the first
    // field guaranteed to be present for every case.
    await expect(this.caseNumber()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Opens a security report analysis. It reuses the same header as a case, so
   * the field locators below apply unchanged.
   *
   * @param projectId - Project the SRA belongs to.
   * @param sraId - SRA sysid.
   */
  async openSecurityReportAnalysis(
    projectId: string,
    sraId: string,
  ): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${CASE_DETAIL.sraPathSegment}/${sraId}`,
    );
    await expect(this.caseNumber()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Opens a service request. It reuses the same header as a case, so the field
   * locators below apply unchanged — bar severity, which service requests do not
   * carry.
   *
   * @param projectId - Project the request belongs to.
   * @param serviceRequestId - Service request sysid.
   */
  async openServiceRequest(
    projectId: string,
    serviceRequestId: string,
  ): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${CASE_DETAIL.serviceRequestPathSegment}/${serviceRequestId}`,
    );
    await expect(this.caseNumber()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Opens an announcement. Like SRAs and service requests it reuses the case
   * detail header, so the field locators below apply unchanged.
   *
   * @param projectId - Project the announcement belongs to.
   * @param announcementId - Announcement sysid.
   */
  async openAnnouncement(
    projectId: string,
    announcementId: string,
  ): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${CASE_DETAIL.announcementPathSegment}/${announcementId}`,
    );
    await expect(this.caseNumber()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  //
  // Header fields. None carry ids, test ids or labels, so rather than guessing
  // at positions in the header row these match on the *shape* of their content —
  // which is both stable against markup changes and self-documenting.
  //

  /** The ServiceNow case number, e.g. CS0441157. */
  caseNumber(): Locator {
    return this.main().getByText(/^CS\d+$/);
  }

  /** The WSO2 case id, e.g. AUTOMATIONTESTCUSSUB-42. */
  wso2CaseId(pattern: RegExp): Locator {
    return this.main().getByText(pattern);
  }

  /** The severity chip, e.g. "S1" or "S4(Query)". */
  severityChip(severity: string): Locator {
    return this.main().getByText(severity, { exact: true });
  }

  /** The case state shown beside the number, e.g. "Open". */
  stateLabel(state: string): Locator {
    return this.main().getByText(state, { exact: true });
  }

  /** The case subject — the header's `variant="h6"` heading. */
  subject(): Locator {
    return this.main().getByRole("heading").first();
  }

  /** A comment on the Activity tab, matched as a substring. */
  comment(text: string): Locator {
    return this.main().getByText(text, { exact: false }).first();
  }

  //
  // Details tab.
  //

  /** Switches to the Details tab and waits for its first section. */
  async openDetailsTab(): Promise<void> {
    await this.page
      .getByRole("tab", { name: CASE_DETAILS_PANEL.tab, exact: true })
      .click();
    await expect(
      this.detailsText(CASE_DETAILS_PANEL.sections.caseOverview),
    ).toHaveCount(1, { timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Text within the page, for asserting a Details-tab label or section is shown.
   *
   * Returns the full match set rather than narrowing with `.first()`: several
   * labels legitimately repeat — the header already shows a status and a
   * severity, for instance — so callers assert on the count being non-zero,
   * which states "this is displayed" without pretending the page has only one.
   *
   * @param text - Exact label or heading text.
   * @returns Locator for every match.
   */
  detailsText(text: string): Locator {
    return this.main().getByText(text, { exact: true });
  }

  //
  // Activity tab — the comment box.
  //

  /** The comment editor. A Lexical contenteditable, so it is typed into rather
   * than filled. */
  commentEditor(): Locator {
    return this.main().getByTestId(CASE_DETAIL.commentEditorTestId);
  }

  sendCommentButton(): Locator {
    return this.main().getByRole("button", {
      name: CASE_COMMENT_INPUT.sendButton,
      exact: true,
    });
  }

  /**
   * Types a comment and sends it, waiting for the POST to land.
   *
   * @param text - Comment body.
   * @returns The create response, for the caller to assert on.
   */
  async addComment(text: string): Promise<Response> {
    const editor = this.commentEditor();
    await expect(editor).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await editor.click();
    await editor.pressSequentially(text);

    // The send control stays disabled until the editor holds submittable
    // content, so this also confirms the text registered with Lexical.
    await expect(this.sendCommentButton()).toBeEnabled();

    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          /\/cases\/[^/]+\/comments$/.test(new URL(r.url()).pathname) &&
          r.request().method() === "POST",
      ),
      this.sendCommentButton().click(),
    ]);
    return response;
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
  async closeCase(): Promise<Response> {
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
