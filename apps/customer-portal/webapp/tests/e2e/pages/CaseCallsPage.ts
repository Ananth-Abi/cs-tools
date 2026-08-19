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
import { CASE_CALLS, CASE_DETAIL } from "../utils/selectors";

/** How long to allow for the tab and its queries to resolve. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for a case's Calls tab and its Request Call modal.
 */
export class CaseCallsPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * The Calls tab itself, whether or not it is rendered.
   *
   * The tab is withheld unless the case's status is one that allows scheduling
   * (CALL_SCHEDULABLE_CASE_STATUSES), so callers assert on its count to check
   * availability rather than assuming it is there.
   */
  tab(): Locator {
    return this.page.getByRole("tab", { name: CASE_CALLS.tab });
  }

  /**
   * The case's other tabs, which are present whatever the status.
   *
   * Exists so that "the Calls tab is absent" can be told apart from "the tab
   * strip has not rendered yet" — without that, the absence assertion would
   * pass on a page that never finished loading.
   */
  alwaysPresentTabs(): Locator {
    return this.page
      .getByRole("tab")
      .filter({ hasText: CASE_CALLS.alwaysPresentTab });
  }

  /**
   * Switches to the Calls tab and waits for the list to resolve.
   *
   * The Request Call button renders while the requests are still loading, so it
   * is not on its own a signal that the list is real — a caller counting cards
   * straight afterwards would read zero. The button is rendered either above the
   * list or inside the empty state, so its presence marks the point where the
   * panel has settled into one of those two states.
   */
  async openTab(): Promise<void> {
    const tab = this.page.getByRole("tab", { name: CASE_CALLS.tab });
    await expect(tab).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await tab.click();
    await expect(this.requestCallButton().first()).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * The Request Call button.
   *
   * Returns every match: the panel renders it above the list, or inside the
   * empty state when there are none, and callers act on the first.
   */
  requestCallButton(): Locator {
    return this.main().getByRole("button", {
      name: CASE_CALLS.requestButton,
      exact: true,
    });
  }

  modal(): Locator {
    return this.page.getByRole("dialog");
  }

  /**
   * Opens the Request Call modal.
   *
   * Fails with a pointed message when the account has no time zone on its
   * profile: CallsPanel then opens a "Time Zone Not Set" dialog instead, and the
   * request modal never appears. That is account setup rather than a bug in the
   * flow under test, so it is worth naming.
   */
  async openRequestModal(): Promise<void> {
    await this.requestCallButton().first().click();

    const modal = this.modal();
    await expect(modal).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await expect(
      modal.getByText(CASE_CALLS.timeZoneDialogTitle),
      "the signed-in account has no time zone set on its profile, so the " +
        "Request Call modal cannot open — set one in profile settings",
    ).toHaveCount(0);
    await expect(modal.getByText(CASE_CALLS.modal.description)).toBeVisible();
  }

  /** The preferred time input, which takes a "YYYY-MM-DDTHH:mm" value. */
  preferredTimeInput(): Locator {
    return this.modal().locator(`#${CASE_CALLS.modal.preferredTimeInputId}`);
  }

  /**
   * A preferred time input by position, for the rows the Add button appends.
   *
   * @param index - Zero-based row.
   */
  preferredTimeInputAt(index: number): Locator {
    return this.modal().locator(`#preferred-time-${index}`);
  }

  /**
   * All preferred time inputs currently on the form.
   *
   * Narrowed to `input`: MUI gives each field's label the id `<id>-label`, so a
   * bare prefix match counts every row twice.
   */
  preferredTimeInputs(): Locator {
    return this.modal().locator('input[id^="preferred-time-"]');
  }

  /** Appends another preferred time row. */
  addPreferredTimeButton(): Locator {
    return this.modal()
      .getByRole("button", { name: CASE_CALLS.modal.addTimeButton, exact: true })
      .first();
  }

  /**
   * The remove control on a preferred time row.
   *
   * @param position - 1-based row, matching the control's accessible name.
   */
  removePreferredTimeButton(position: number): Locator {
    return this.modal().getByRole("button", {
      name: CASE_CALLS.modal.removeTimeButton(position),
      exact: true,
    });
  }

  /**
   * Opens the duration select and reads the options it offers.
   *
   * Closes it again with Escape, so the caller is left with the form as it was
   * rather than an open listbox covering the buttons it wants to assert on.
   *
   * Waits for the first option before reading them. `allInnerTexts` resolves
   * immediately against whatever matches at that moment — it does not retry like
   * an assertion — so reading straight after the click can return an empty list
   * while the portal is still mounting, and the caller would compare that empty
   * list against the expected options.
   *
   * @returns The option labels, in order.
   */
  async durationOptionLabels(): Promise<string[]> {
    await this.durationSelect().click();

    const options = this.page.getByRole("option");
    await expect(options.first()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    const labels = await options.allInnerTexts();

    await this.page.keyboard.press("Escape");
    return labels.map((label) => label.trim());
  }

  reasonInput(): Locator {
    return this.modal().getByLabel(CASE_CALLS.modal.reasonLabel, {
      exact: true,
    });
  }

  /**
   * Fills the request form.
   *
   * The modal seeds the preferred time with the earliest allowed value on open,
   * so this overwrites it rather than typing into an empty field. The duration
   * is left at its default.
   *
   * @param preferredTimeLocal - Value for the datetime-local input.
   * @param reason - Reason for the call.
   */
  async fillRequest(preferredTimeLocal: string, reason: string): Promise<void> {
    await this.preferredTimeInput().fill(preferredTimeLocal);
    await this.reasonInput().fill(reason);
  }

  /** The modal's submit button, scoped to the dialog — the button that opened
   * the modal carries the same label and is still in the DOM behind it. */
  submitButton(): Locator {
    return this.modal().getByRole("button", {
      name: CASE_CALLS.modal.submitButton,
      exact: true,
    });
  }

  /**
   * Submits the request and waits for the create POST to land.
   *
   * Waits for the response whatever its status, then leaves the caller to assert
   * on it: requiring 2xx in the predicate would mean a rejected create never
   * matches and the test times out with no clue as to why.
   *
   * @returns The create response.
   */
  async submit(): Promise<Response> {
    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          /\/cases\/[^/]+\/call-requests$/.test(new URL(r.url()).pathname) &&
          r.request().method() === "POST",
        { timeout: LOAD_TIMEOUT_MS },
      ),
      this.submitButton().click(),
    ]);
    return response;
  }

  /**
   * The card for a request, found by its reason.
   *
   * Reasons come back from the backend with a "[Customer] " prefix on some
   * tenants, so this matches on the reason as a substring rather than exactly.
   *
   * A cancelled request leaves the list entirely rather than staying on with a
   * spent state — the panel searches only the non-cancelled states — so this
   * needs no narrowing to avoid picking up a dead record.
   *
   * @param reason - Reason the request was filed with.
   * @returns Locator for the card.
   */
  card(reason: string): Locator {
    return this.main()
      .locator("div")
      .filter({ has: this.page.getByText(CASE_CALLS.card.title, { exact: true }) })
      .filter({ hasText: reason })
      .last();
  }

  /** Whether a request with this reason is already on the tab. */
  async hasRequest(reason: string): Promise<boolean> {
    return (await this.main().getByText(reason, { exact: false }).count()) > 0;
  }

  /** A card's Reschedule button, which reopens the modal in edit mode. */
  rescheduleButton(reason: string): Locator {
    return this.card(reason).getByRole("button", {
      name: CASE_CALLS.card.rescheduleButton,
      exact: true,
    });
  }

  /**
   * Opens the Edit Call Request modal from a card.
   *
   * Asserts the title, which is what tells edit mode apart from create — both
   * are the same component and the same dialog role, so without this a failure
   * to enter edit mode would only surface later as a confusing missing-field
   * error.
   *
   * @param reason - Reason of the request to reschedule.
   */
  async openEditModal(reason: string): Promise<void> {
    const reschedule = this.rescheduleButton(reason);
    await expect(
      reschedule,
      "Reschedule is disabled once a request reaches a terminal state",
    ).toBeEnabled({ timeout: LOAD_TIMEOUT_MS });
    await reschedule.click();

    const modal = this.modal();
    await expect(modal).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await expect(modal.getByText(CASE_CALLS.editModal.title)).toBeVisible();
    await expect(
      modal.getByText(CASE_CALLS.editModal.description),
    ).toBeVisible();
  }

  /**
   * The Meeting Duration select.
   *
   * The modal's only combobox — the preferred time is a native datetime input —
   * so it needs no further narrowing. MUI does not tie the label to the control
   * with `for`, which rules out `getByLabel`.
   */
  durationSelect(): Locator {
    return this.modal().getByRole("combobox");
  }

  /**
   * Picks a meeting duration.
   *
   * The option list renders in a portal at the document root rather than inside
   * the dialog, so it is looked up page-wide.
   *
   * @param option - Exact option label, e.g. "1 hour".
   */
  async selectDuration(option: string): Promise<void> {
    await this.durationSelect().click();
    await this.page.getByRole("option", { name: option, exact: true }).click();
  }

  /**
   * The modal's Cancel button, which dismisses it without saving.
   *
   * Named for what it does to the dialog, not to the call — a card's Cancel
   * button cancels the request itself, which is a different action entirely.
   */
  modalCancelButton(): Locator {
    return this.modal().getByRole("button", {
      name: CASE_CALLS.modal.cancelButton,
      exact: true,
    });
  }

  /**
   * Dismisses the modal without saving and waits for it to close.
   */
  async dismissModal(): Promise<void> {
    await this.modalCancelButton().click();
    await expect(this.modal()).toBeHidden({ timeout: LOAD_TIMEOUT_MS });
  }

  /** The edit modal's submit button, scoped to the dialog. */
  updateButton(): Locator {
    return this.modal().getByRole("button", {
      name: CASE_CALLS.editModal.submitButton,
      exact: true,
    });
  }

  /**
   * A card's Cancel button.
   *
   * @param reason - Reason of the request.
   */
  cancelButton(reason: string): Locator {
    return this.card(reason).getByRole("button", {
      name: CASE_CALLS.card.cancelButton,
      exact: true,
    });
  }

  /**
   * Opens the cancellation dialog from a card.
   *
   * @param reason - Reason of the request to cancel.
   */
  async openCancelModal(reason: string): Promise<void> {
    const cancel = this.cancelButton(reason);
    await expect(
      cancel,
      "Cancel is disabled once a request reaches a terminal state",
    ).toBeEnabled({ timeout: LOAD_TIMEOUT_MS });
    await cancel.click();

    const modal = this.modal();
    await expect(modal).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await expect(modal).toContainText(CASE_CALLS.cancelModal.title);
  }

  /** The cancellation dialog's own Reason field, which is required. */
  cancellationReasonInput(): Locator {
    return this.modal().locator(`#${CASE_CALLS.cancelModal.reasonInputId}`);
  }

  /**
   * Confirms the cancellation and waits for the PATCH to land.
   *
   * Cancelling goes through the same endpoint as a reschedule — it is a state
   * change, not a delete — so the request body is what tells the two apart.
   *
   * @param cancellationReason - Why the call is being cancelled.
   * @returns The cancellation response.
   */
  async confirmCancel(cancellationReason: string): Promise<Response> {
    await this.cancellationReasonInput().fill(cancellationReason);

    const confirm = this.modal().getByRole("button", {
      name: CASE_CALLS.cancelModal.confirmButton,
      exact: true,
    });
    await expect(
      confirm,
      "Confirm stays disabled until a cancellation reason is given",
    ).toBeEnabled();

    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          /\/cases\/[^/]+\/call-requests\/[^/]+$/.test(
            new URL(r.url()).pathname,
          ) && r.request().method() === "PATCH",
        { timeout: LOAD_TIMEOUT_MS },
      ),
      confirm.click(),
    ]);
    return response;
  }

  /**
   * Submits the edit and waits for the update PATCH to land.
   *
   * Waits whatever the status, for the same reason as `submit()`: a rejected
   * update that never matched the predicate would time out unexplained.
   *
   * @returns The update response.
   */
  async update(): Promise<Response> {
    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          /\/cases\/[^/]+\/call-requests\/[^/]+$/.test(
            new URL(r.url()).pathname,
          ) && r.request().method() === "PATCH",
        { timeout: LOAD_TIMEOUT_MS },
      ),
      this.updateButton().click(),
    ]);
    return response;
  }
}
