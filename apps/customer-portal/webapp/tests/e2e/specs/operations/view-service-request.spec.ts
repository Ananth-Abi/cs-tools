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
// Opens existing service requests and checks the header shows what it should.
//
// ✅ READ-ONLY. Requests are opened by URL and asserted; nothing is created or
// modified, so this is safe to run repeatedly.
//
// A service request reuses the case detail header, so the same locators apply —
// but it carries no severity chip, since the service request form has no severity
// field.
//
// What is and is not pinned:
//
// - **Subject** is pinned, and is the Request Details value the request was
//   raised with — so it comes from SERVICE_REQUEST_INPUT rather than a duplicated
//   string. If the create spec's input changes, this fixture follows it.
// - **WSO2 case id** and **case number** are asserted by format, since the values
//   differ per record. The id prefix is per project.
// - **State** must be one of the real case states, so a blank or placeholder
//   fails.
//
// Only Managed Cloud Subscription has an entry: it is the one project whose
// service request coverage exists so far.
//

import { test, expect, withSession } from "../../fixtures/test";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import {
  CASE_VIEW_EXPECTATIONS,
  PROJECTS,
  SERVICE_REQUEST_VIEWS,
} from "../../config/testData";
import { CASE_DETAIL } from "../../utils/selectors";

withSession(test);

test.describe("View Service Request", () => {
  // Each request is a cold page load with several queries behind the header; the
  // 30s default is not enough.
  test.describe.configure({ timeout: 120_000 });

  for (const {
    projectType,
    serviceRequestId,
    subject,
    wso2CaseIdPattern,
  } of SERVICE_REQUEST_VIEWS) {
    const project = PROJECTS[projectType];

    test.describe(projectType, () => {
      test("shows details for a service request", async ({ page }) => {
        test.skip(
          !project.id || !serviceRequestId,
          `${projectType} needs a project id and a service request id. ` +
            `Fill them in tests/e2e/config/testData.ts.`,
        );

        const serviceRequest = new CaseDetailPage(page);
        await serviceRequest.openServiceRequest(project.id, serviceRequestId);

        // Case number — format only; the value differs per record.
        await expect(serviceRequest.caseNumber()).toBeVisible();
        await expect(serviceRequest.caseNumber()).toHaveText(
          CASE_VIEW_EXPECTATIONS.caseNumberPattern,
        );

        // WSO2 case id — project-specific prefix plus an integer.
        await expect(
          serviceRequest.wso2CaseId(wso2CaseIdPattern),
        ).toBeVisible();

        // State — must be one of the real states rather than blank or a dash.
        const states = CASE_DETAIL.header.states;
        const visibleStates = await Promise.all(
          states.map((state) => serviceRequest.stateLabel(state).count()),
        );
        expect(
          visibleStates.reduce((total, n) => total + n, 0),
          `expected one of ${states.join(", ")} to be shown as the state`,
        ).toBeGreaterThan(0);

        // Subject — the Request Details value the request was raised with.
        await expect(serviceRequest.subject()).toHaveText(subject);
      });
    });
  }
});
