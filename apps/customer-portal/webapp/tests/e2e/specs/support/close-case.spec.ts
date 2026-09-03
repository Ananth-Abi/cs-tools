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
// Closes a support case from its detail page.
//
// The case is created by the test rather than picked from the project's
// existing cases: closing is destructive and irreversible from the portal, so
// the test must only ever act on a record it owns. Creation lands on
// `/projects/:projectId/support/cases/:caseId`, which is where the Close action
// lives.
//
// Scoped to the Subscription project. Closing is not project-type specific, but
// exercising it on one project keeps the number of permanent records these
// specs leave behind to a minimum.
//
// "Close" is the present-tense rendering of the `Closed` action that
// `getAvailableCaseActions` offers for an open case, and it is
// confirmation-gated — see CaseStateConfirmDialog.
//

import { test, expect, withSession } from "../../fixtures/test";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import { CASE_INPUT, PROJECTS, ProjectType } from "../../config/testData";
import {
  createCaseViaGetHelp,
  skipWhenUnconfigured,
} from "../../utils/caseFlows";
import { CASE_DETAIL } from "../../utils/selectors";

withSession(test);

test.describe("Close Case", () => {
  // Creates a case first, so this needs the create flow's budget on top of its
  // own.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[ProjectType.SUBSCRIPTION];
  const caseInput = CASE_INPUT[ProjectType.SUBSCRIPTION];

  test(`${ProjectType.SUBSCRIPTION} — closes a newly created case`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    const created = await createCaseViaGetHelp(page, project, caseInput);
    console.log(
      `Created case to close (${ProjectType.SUBSCRIPTION}): ` +
        `${created.number ?? created.id}`,
    );

    // Creation must have landed on the case detail page — that is where the
    // Close action lives, and the rest of this test depends on it.
    await expect(page).toHaveURL(new RegExp(CASE_DETAIL.pathSegment));

    const caseDetail = new CaseDetailPage(page);
    await caseDetail.closeCase();

    // The action row rebuilds from the new status: an open case offers "Close",
    // a closed one does not (it offers "Open Related Case" instead), so the
    // button disappearing is the UI's own confirmation that the state changed.
    await expect(caseDetail.closeButton()).toBeHidden();
    await expect(caseDetail.closedStatusChip()).toBeVisible();

    console.log(`Closed case: ${created.number ?? created.id}`);
  });
});
