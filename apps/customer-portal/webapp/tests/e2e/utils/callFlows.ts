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
// Call-request flows shared by the happy-path suite and the validation suite.
// Both need a case's Calls tab open, and both need a request to exist before
// they can exercise anything that acts on one.
//

import { expect, type Page } from "../fixtures/test";
import { CaseCallsPage } from "../pages/CaseCallsPage";
import { CaseDetailPage } from "../pages/CaseDetailPage";
import { CALL_REQUEST_INPUT, PROJECTS } from "../config/testData";
import { expectSuccess } from "./caseFlows";

/**
 * A future date at a given time, formatted for a `datetime-local` input.
 *
 * Built from local date parts rather than an ISO string, because `toISOString`
 * is UTC and would shift the date by a day either side of midnight for anyone
 * not on UTC.
 *
 * @param days - How many days ahead: 1 for tomorrow, 2 for the day after.
 * @param timeOfDay - "HH:mm" to request.
 * @returns A "YYYY-MM-DDTHH:mm" value.
 */
export function daysFromNowAt(days: number, timeOfDay: string): string {
  const date = new Date();
  date.setDate(date.getDate() + days);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}T${timeOfDay}`;
}

/**
 * Opens the configured case's Calls tab.
 *
 * @param page - Test page.
 * @returns The tab's page object.
 */
export async function openCallsTab(page: Page): Promise<CaseCallsPage> {
  const project = PROJECTS[CALL_REQUEST_INPUT.projectType];
  const caseDetail = new CaseDetailPage(page);
  await caseDetail.open(project.id, CALL_REQUEST_INPUT.caseId);
  const calls = new CaseCallsPage(page);
  await calls.openTab();
  return calls;
}

/**
 * Makes sure a call request with the configured reason exists, filing one only
 * when it does not.
 *
 * ⚠️ May create a permanent record — a call request cannot be removed, only
 * cancelled. The guard on the reason is what stops it stacking one per run.
 *
 * Shared rather than owned by one spec so that every test needing a request to
 * act on can get one itself, instead of depending on another test having run
 * first — which holds in a whole-file run but not when a test runs alone.
 *
 * @param calls - The case's open Calls tab.
 * @returns Whether a request was already there.
 */
export async function ensureCallRequest(
  calls: CaseCallsPage,
): Promise<boolean> {
  if (await calls.hasRequest(CALL_REQUEST_INPUT.reason)) {
    console.log(
      `${CALL_REQUEST_INPUT.projectType}: a call request with this reason ` +
        `already exists, asserting it rather than filing another`,
    );
    return true;
  }

  const preferredTime = daysFromNowAt(1, CALL_REQUEST_INPUT.preferredTimeOfDay);
  await calls.openRequestModal();
  await calls.fillRequest(preferredTime, CALL_REQUEST_INPUT.reason);

  // The modal seeds the field with the earliest allowed time on open, so this
  // confirms the value was actually replaced with tomorrow's — and that the
  // input did not reject it against its own `min`.
  await expect(calls.preferredTimeInput()).toHaveValue(preferredTime);
  await expect(calls.submitButton()).toBeEnabled();

  const response = await calls.submit();
  await expectSuccess(response, "create call request");

  // The modal closes itself on success, so it staying open would mean the
  // request was accepted but the UI never moved on.
  await expect(calls.modal()).toBeHidden();
  return false;
}
