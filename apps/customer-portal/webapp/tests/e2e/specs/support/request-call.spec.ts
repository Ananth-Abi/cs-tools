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

//
// Requesting a call from a case's Calls tab.
//
// ⚠️ Writes to a REAL backend via POST /cases/{id}/call-requests. A call request
// cannot be removed — the delete action cancels it and the record stays — so the
// spec files one only when its own earlier request is not already on the tab,
// keyed on the fixed reason text. Every later run asserts the existing card
// instead of stacking another.
//
// That guard is also why the preferred time is only checked against tomorrow on
// the run that actually created the request: on later runs the card carries
// whichever date that first run picked.
//
// The second describe below covers field gating across all three dialogs and
// writes NOTHING — every scenario stops at a disabled control or dismisses the
// dialog, and each test counts the writes that left the browser to prove it.
//

import { test, expect, withSession, type Page } from "../../fixtures/test";
import { CALL_REQUEST_INPUT, PROJECTS } from "../../config/testData";
import { CASE_CALLS } from "../../utils/selectors";
import { expectSuccess } from "../../utils/caseFlows";
import { CaseCallsPage } from "../../pages/CaseCallsPage";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import {
  daysFromNowAt,
  ensureCallRequest,
  openCallsTab,
} from "../../utils/callFlows";

withSession(test);

/**
 * Counts the call-request writes a test causes, so "nothing was submitted" can
 * be asserted rather than assumed.
 *
 * @param page - Test page.
 * @returns A reader for the count so far.
 */
function countCallRequestWrites(page: Page): () => number {
  let writes = 0;
  page.on("request", (request) => {
    const path = new URL(request.url()).pathname;
    const method = request.method();

    // Only the two write endpoints. Reading the list is *also* a POST — to
    // `/call-requests/search` — so matching "POST to anything under
    // call-requests" counts every refetch as a write, which is exactly the
    // mistake that made this counter report 3 instead of 0.
    const isCreate = method === "POST" && /\/call-requests$/.test(path);
    const isUpdate =
      method === "PATCH" && /\/call-requests\/[^/]+$/.test(path);

    if (isCreate || isUpdate) writes += 1;
  });
  return () => writes;
}

test.describe("Request Call", () => {
  // A cold case load, a tab switch, the call-requests query and a modal whose
  // fields depend on the user's profile — well past the 30s default.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[CALL_REQUEST_INPUT.projectType];

  test.describe(CALL_REQUEST_INPUT.projectType, () => {
    test("requests a call from the case's Calls tab", async ({ page }) => {
      test.skip(
        !project.id || !CALL_REQUEST_INPUT.caseId,
        `${CALL_REQUEST_INPUT.projectType} needs a project id and a case id. ` +
          `Fill them in tests/e2e/config/testData.ts.`,
      );

      const calls = await openCallsTab(page);
      const alreadyRequested = await ensureCallRequest(calls);

      // The card, whichever run created it.
      const card = calls.card(CALL_REQUEST_INPUT.reason);
      await expect(card).toBeVisible();
      await expect(card).toContainText(CALL_REQUEST_INPUT.reason);
      await expect(card).toContainText(CASE_CALLS.card.durationLabel);
      await expect(card).toContainText(CASE_CALLS.card.preferredTimesLabel);

      // The default duration only holds on the run that filed the request. The
      // reschedule test below moves an existing one to an hour, and the record
      // persists between runs — so a later run finds 60 minutes here, which is
      // correct rather than a regression.
      await expect(card).toContainText(
        alreadyRequested
          ? CASE_CALLS.card.durationPattern
          : CALL_REQUEST_INPUT.durationLabel,
      );

      // A newly filed request opens pending on WSO2. Soft, because a request
      // left by an earlier run may since have been actioned by a real engineer,
      // which is not a failure of this flow.
      await expect
        .soft(card, "a new call request should be pending on WSO2")
        .toContainText(CASE_CALLS.card.pendingState);

      // The requested time is rendered in the user's profile time zone, so the
      // exact string is not reconstructable here — what matters is that a time
      // came back at all rather than the placeholder.
      await expect(card).not.toContainText(
        `${CASE_CALLS.card.preferredTimesLabel} ${CASE_CALLS.card.emptyValue}`,
      );

      console.log(
        `Call request (${CALL_REQUEST_INPUT.projectType}): ` +
          `${alreadyRequested ? "existing" : "created"}`,
      );
    });

    test("reschedules the call request", async ({ page }) => {
      test.skip(
        !project.id || !CALL_REQUEST_INPUT.caseId,
        `${CALL_REQUEST_INPUT.projectType} needs a project id and a case id.`,
      );

      const calls = await openCallsTab(page);
      await ensureCallRequest(calls);

      await calls.openEditModal(CALL_REQUEST_INPUT.reason);

      const rescheduledTime = daysFromNowAt(
        2,
        CALL_REQUEST_INPUT.preferredTimeOfDay,
      );
      await calls.preferredTimeInput().fill(rescheduledTime);
      await expect(calls.preferredTimeInput()).toHaveValue(rescheduledTime);

      await calls.selectDuration(CALL_REQUEST_INPUT.rescheduledDurationOption);

      const response = await calls.update();
      await expectSuccess(response, "update call request");

      // The duration on the wire, which is free of the time zone problem that
      // stops the rendered times from being checked exactly. The modal calls it
      // "1 hour"; the payload is minutes.
      const payload = JSON.parse(
        response.request().postData() ?? "{}",
      ) as { durationInMinutes?: number; utcTimes?: string[] };
      expect(payload.durationInMinutes).toBe(60);
      expect(
        payload.utcTimes?.length,
        "the update should carry the rescheduled time",
      ).toBeGreaterThan(0);

      await expect(calls.modal()).toBeHidden();

      // The card picks the change up, which is what makes this a reschedule
      // rather than an accepted request that never reached the list.
      const card = calls.card(CALL_REQUEST_INPUT.reason);
      await expect(card).toBeVisible();
      await expect(card).toContainText(
        CALL_REQUEST_INPUT.rescheduledDurationLabel,
      );
      await expect(card).not.toContainText(
        `${CASE_CALLS.card.preferredTimesLabel} ${CASE_CALLS.card.emptyValue}`,
      );

      console.log(
        `Call request (${CALL_REQUEST_INPUT.projectType}): rescheduled to ` +
          `${rescheduledTime} for ${CALL_REQUEST_INPUT.rescheduledDurationLabel}`,
      );
    });

    test("discards a reschedule when the modal is cancelled", async ({
      page,
    }) => {
      test.skip(
        !project.id || !CALL_REQUEST_INPUT.caseId,
        `${CALL_REQUEST_INPUT.projectType} needs a project id and a case id.`,
      );

      // Counted from before the modal opens: dismissing must not send anything,
      // and an assertion on the card alone could not tell "nothing was saved"
      // apart from "saved the same values back".
      let updateRequests = 0;
      page.on("request", (request) => {
        if (
          request.method() === "PATCH" &&
          /\/cases\/[^/]+\/call-requests\/[^/]+$/.test(
            new URL(request.url()).pathname,
          )
        ) {
          updateRequests += 1;
        }
      });

      const calls = await openCallsTab(page);
      await ensureCallRequest(calls);

      // Snapshot the card, so the comparison afterwards covers every field
      // rather than the couple this test thought to name.
      const card = calls.card(CALL_REQUEST_INPUT.reason);
      await expect(card).toBeVisible();
      const before = await card.innerText();

      await calls.openEditModal(CALL_REQUEST_INPUT.reason);

      // Edit the form before dismissing. Without a pending change, the dialog
      // closing proves nothing: there would be nothing for it to discard.
      const discardedTime = daysFromNowAt(
        3,
        CALL_REQUEST_INPUT.preferredTimeOfDay,
      );
      await calls.preferredTimeInput().fill(discardedTime);
      await expect(calls.preferredTimeInput()).toHaveValue(discardedTime);

      await calls.dismissModal();

      expect(
        updateRequests,
        "cancelling the modal should not send an update",
      ).toBe(0);

      // The card is exactly as it was — the edit went nowhere. Compared through
      // innerText on both sides: toHaveText normalises whitespace differently
      // from innerText, so identical content fails against a snapshot taken
      // this way.
      await expect
        .poll(async () => (await card.innerText()).trim(), {
          message: "the card should be unchanged after dismissing the modal",
          timeout: 10_000,
        })
        .toBe(before.trim());

      console.log(
        `Call request (${CALL_REQUEST_INPUT.projectType}): reschedule to ` +
          `${discardedTime} discarded`,
      );
    });

    test("cancels a call request", async ({ page }) => {
      test.skip(
        !project.id || !CALL_REQUEST_INPUT.caseId,
        `${CALL_REQUEST_INPUT.projectType} needs a project id and a case id.`,
      );

      const calls = await openCallsTab(page);

      // Files its own request rather than reusing the one above: cancelling is
      // terminal, and taking the shared request out would leave the reschedule
      // test with a disabled button. Unguarded on purpose — a cancelled request
      // cannot be cancelled twice, so each run needs a live one of its own.
      //
      // The reason is stamped per run because cards are matched on it as a
      // substring. A run that dies between creating and cancelling leaves a
      // pending request behind; with a fixed reason the next run would have two
      // cards matching, act on whichever `.last()` resolved to, and then fail its
      // closing absence check on the other one — permanently, until someone
      // cleaned up by hand. Milliseconds are kept so two runs starting in the
      // same second cannot collide either.
      const cancelReason =
        `${CALL_REQUEST_INPUT.cancel.reason} ` +
        `${new Date().toISOString().slice(0, 23).replace(/[:T.]/g, "-")}`;

      const preferredTime = daysFromNowAt(
        1,
        CALL_REQUEST_INPUT.preferredTimeOfDay,
      );
      await calls.openRequestModal();
      await calls.fillRequest(preferredTime, cancelReason);
      await expect(calls.preferredTimeInput()).toHaveValue(preferredTime);

      const created = await calls.submit();
      await expectSuccess(created, "create call request to cancel");
      await expect(calls.modal()).toBeHidden();

      const live = calls.card(cancelReason);
      await expect(live).toBeVisible();
      await expect(live).toContainText(CASE_CALLS.card.pendingState);

      await calls.openCancelModal(cancelReason);
      const cancelled = await calls.confirmCancel(
        CALL_REQUEST_INPUT.cancel.cancellationReason,
      );
      await expectSuccess(cancelled, "cancel call request");

      // Cancelling reuses the update endpoint, so the body is the only thing
      // that distinguishes it from a reschedule: state 6 is the cancelled state
      // (CALL_REQUEST_STATE_CANCELLED).
      const payload = JSON.parse(cancelled.request().postData() ?? "{}") as {
        stateKey?: number;
        cancellationReason?: string;
      };
      expect(payload.stateKey).toBe(6);
      expect(payload.cancellationReason).toBe(
        CALL_REQUEST_INPUT.cancel.cancellationReason,
      );

      await expect(calls.modal()).toBeHidden();

      // The request leaves the list rather than staying on with a cancelled
      // state: the panel searches only the non-cancelled states, so a cancelled
      // request is filtered out of its own tab. Its disappearance is therefore
      // what proves the cancellation took, and it is also why these records do
      // not pile up on the tab run after run.
      await expect(calls.card(cancelReason)).toHaveCount(0);

      console.log(
        `Call request (${CALL_REQUEST_INPUT.projectType}): created for ` +
          `${preferredTime} and cancelled (${cancelReason})`,
      );
    });
  });
});

test.describe("Request Call — validation", () => {
  // A cold case load, a tab switch and the call-requests query, before any
  // dialog is even opened — well past the 30s default.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[CALL_REQUEST_INPUT.projectType];

  test.describe(CALL_REQUEST_INPUT.projectType, () => {
    test.beforeEach(() => {
      test.skip(
        !project.id || !CALL_REQUEST_INPUT.caseId,
        `${CALL_REQUEST_INPUT.projectType} needs a project id and a case id. ` +
          `Fill them in tests/e2e/config/testData.ts.`,
      );
    });

    test("keeps submit disabled until a reason is given", async ({ page }) => {
      const writes = countCallRequestWrites(page);
      const calls = await openCallsTab(page);
      await calls.openRequestModal();

      // The preferred time is seeded on open, so the reason is the only thing
      // still missing — which makes this a clean test of that one rule.
      await expect(calls.preferredTimeInput()).not.toHaveValue("");
      await expect(calls.submitButton()).toBeDisabled();

      await calls.reasonInput().fill(CALL_REQUEST_INPUT.reason);
      await expect(calls.submitButton()).toBeEnabled();

      // Whitespace is not a reason: handleSubmit trims before checking.
      await calls.reasonInput().fill("   ");
      await expect(calls.submitButton()).toBeDisabled();

      await calls.dismissModal();
      expect(writes(), "validation must not submit anything").toBe(0);
    });

    test("keeps submit disabled when a preferred time is cleared", async ({
      page,
    }) => {
      const writes = countCallRequestWrites(page);
      const calls = await openCallsTab(page);
      await calls.openRequestModal();

      await calls.reasonInput().fill(CALL_REQUEST_INPUT.reason);
      await expect(calls.submitButton()).toBeEnabled();

      await calls.preferredTimeInput().fill("");
      await expect(calls.submitButton()).toBeDisabled();

      await calls.dismissModal();
      expect(writes(), "validation must not submit anything").toBe(0);
    });

    test("refuses a preferred time earlier than the allowed floor", async ({
      page,
    }) => {
      const writes = countCallRequestWrites(page);
      const calls = await openCallsTab(page);
      await calls.openRequestModal();

      // The floor is computed from the case severity's allocation time, so it
      // is read off the input rather than recomputed here — the test asserts the
      // rule is enforced, not what the tenant's allocation happens to be.
      const input = calls.preferredTimeInput();
      const floor = await input.getAttribute("min");
      expect(floor, "the input should carry a min").toBeTruthy();

      // A date well in the past, which the change handler snaps back up.
      await input.fill(daysFromNowAt(-7, CALL_REQUEST_INPUT.preferredTimeOfDay));
      await expect(input).toHaveValue(floor as string);

      await calls.dismissModal();
      expect(writes(), "validation must not submit anything").toBe(0);
    });

    test("offers up to three preferred times and keeps the first", async ({
      page,
    }) => {
      const writes = countCallRequestWrites(page);
      const calls = await openCallsTab(page);
      await calls.openRequestModal();

      await expect(calls.preferredTimeInputs()).toHaveCount(1);

      // The first row can never be removed — a request must keep at least one
      // preferred time.
      await expect(calls.removePreferredTimeButton(1)).toBeDisabled();

      for (let count = 2; count <= CASE_CALLS.modal.maxPreferredTimes; count++) {
        await calls.addPreferredTimeButton().click();
        await expect(calls.preferredTimeInputs()).toHaveCount(count);
      }

      // At the maximum the control is withheld rather than silently ignoring
      // the click.
      await expect(calls.addPreferredTimeButton()).toBeDisabled();

      // Added rows are seeded with the floor, so the form stays submittable.
      await expect(
        calls.preferredTimeInputAt(CASE_CALLS.modal.maxPreferredTimes - 1),
      ).not.toHaveValue("");

      // And they can be taken back off.
      await calls
        .removePreferredTimeButton(CASE_CALLS.modal.maxPreferredTimes)
        .click();
      await expect(calls.preferredTimeInputs()).toHaveCount(
        CASE_CALLS.modal.maxPreferredTimes - 1,
      );
      await expect(calls.addPreferredTimeButton()).toBeEnabled();

      await calls.dismissModal();
      expect(writes(), "validation must not submit anything").toBe(0);
    });

    test("offers exactly the four meeting durations", async ({ page }) => {
      const writes = countCallRequestWrites(page);
      const calls = await openCallsTab(page);
      await calls.openRequestModal();

      expect(await calls.durationOptionLabels()).toEqual([
        ...CASE_CALLS.modal.durationOptions,
      ]);

      await calls.dismissModal();
      expect(writes(), "validation must not submit anything").toBe(0);
    });

    test("does not offer the reason field when rescheduling", async ({
      page,
    }) => {
      const writes = countCallRequestWrites(page);
      const calls = await openCallsTab(page);
      const created = await ensureCallRequest(calls);

      await calls.openEditModal(CALL_REQUEST_INPUT.reason);

      // A reschedule may change times and duration only; the reason the call was
      // asked for is fixed once filed.
      await expect(calls.reasonInput()).toHaveCount(0);
      await expect(calls.durationSelect()).toBeVisible();

      await calls.dismissModal();

      // One write is allowed here, and only if the shared request had to be
      // filed first — the dialog itself must send nothing.
      expect(writes()).toBe(created ? 0 : 1);
    });

    test("keeps update disabled when a preferred time is cleared", async ({
      page,
    }) => {
      const writes = countCallRequestWrites(page);
      const calls = await openCallsTab(page);
      const created = await ensureCallRequest(calls);

      await calls.openEditModal(CALL_REQUEST_INPUT.reason);
      await expect(calls.updateButton()).toBeEnabled();

      await calls.preferredTimeInput().fill("");
      await expect(calls.updateButton()).toBeDisabled();

      await calls.dismissModal();
      expect(writes()).toBe(created ? 0 : 1);
    });

    test("keeps confirm disabled until a cancellation reason is given", async ({
      page,
    }) => {
      const writes = countCallRequestWrites(page);
      const calls = await openCallsTab(page);
      const created = await ensureCallRequest(calls);

      await calls.openCancelModal(CALL_REQUEST_INPUT.reason);

      const confirm = calls
        .modal()
        .getByRole("button", {
          name: CASE_CALLS.cancelModal.confirmButton,
          exact: true,
        });
      await expect(confirm).toBeDisabled();

      await calls
        .cancellationReasonInput()
        .fill(CALL_REQUEST_INPUT.cancel.cancellationReason);
      await expect(confirm).toBeEnabled();

      // Whitespace does not count as a reason either.
      await calls.cancellationReasonInput().fill("   ");
      await expect(confirm).toBeDisabled();

      // Go Back leaves the request alone — the point of this test is that the
      // shared request survives it.
      await calls
        .modal()
        .getByRole("button", {
          name: CASE_CALLS.cancelModal.goBackButton,
          exact: true,
        })
        .click();
      await expect(calls.modal()).toBeHidden();

      await expect(calls.card(CALL_REQUEST_INPUT.reason)).toBeVisible();
      expect(writes()).toBe(created ? 0 : 1);
    });
  });
});

test.describe("Calls tab availability", () => {
  // A cold case load per test, twice over — well past the 30s default.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[CALL_REQUEST_INPUT.projectType];

  // Scheduling is offered only while a case is in one of
  // CALL_SCHEDULABLE_CASE_STATUSES. Outside those the Calls tab is not rendered
  // at all — it is withheld rather than shown empty — so there is nowhere to
  // request a call from.
  for (const [index, caseId] of CALL_REQUEST_INPUT.callsUnavailableCaseIds.entries()) {
    test(`withholds the Calls tab on a non-schedulable case (${index + 1})`, async ({
      page,
    }) => {
      test.skip(
        !project.id || !caseId,
        `${CALL_REQUEST_INPUT.projectType} needs a project id and a case id. ` +
          `Fill them in tests/e2e/config/testData.ts.`,
      );

      const caseDetail = new CaseDetailPage(page);
      await caseDetail.open(project.id, caseId);

      const calls = new CaseCallsPage(page);

      // The tab strip has to be on screen first: without this, "no Calls tab"
      // would also be true of a page that never finished loading, and the test
      // would pass for the wrong reason.
      await expect(calls.alwaysPresentTabs().first()).toBeVisible();

      await expect(
        calls.tab(),
        "this case is not in a call-schedulable status, so it should have no " +
          "Calls tab",
      ).toHaveCount(0);

      // And the reason it is absent, so a tab that disappeared for some
      // unrelated cause could not pass itself off as this rule working.
      for (const status of CASE_CALLS.schedulableStatuses) {
        await expect
          .soft(
            caseDetail.stateLabel(status),
            `a case with no Calls tab should not be "${status}"`,
          )
          .toHaveCount(0);
      }

      console.log(`Calls tab withheld on ${caseId}`);
    });
  }
});
