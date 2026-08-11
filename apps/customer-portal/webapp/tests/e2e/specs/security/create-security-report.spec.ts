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
// Raises a security report from the "Get Help" split button's dropdown.
//
// Scoped to the Managed Cloud Subscription project: the Security Report menu
// item only appears when the project has SRA write access
// (`isSecurityReportVisible` in GetHelpDropdown.tsx), which is a per-project
// feature flag rather than something every type has.
//
// A security report is a case raised at /support/security-report/create. The
// same CreateCasePage backs it, with `isSecurityReport` derived from the path —
// so Issue Type and Severity are hidden, an attachment is mandatory, and the
// Title is generated from the deployment, product and today's date rather than
// entered.
//
// ⚠️ Writes to a REAL backend and leaves a permanent record on every run —
// there is no delete counterpart. The configured description is deliberately
// self-describing so the records stay identifiable.
//

import { test, expect, withSession } from "../../fixtures/test";
import { SecurityReportCreatePage } from "../../pages/SecurityReportCreatePage";
import {
  PROJECTS,
  ProjectType,
  SECURITY_REPORT_INPUT,
} from "../../config/testData";
import { isSuccess, skipWhenUnconfigured } from "../../utils/caseFlows";

withSession(test);

test.describe("Security Report", () => {
  // Spans a dashboard load, a menu navigation, two backend-populated dropdowns
  // and a file upload; the 30s default is not enough.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[ProjectType.MANAGED_CLOUD_SUBSCRIPTION];

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — creates a security report`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    const form = new SecurityReportCreatePage(page);

    await form.openViaGetHelpMenu(project.id);

    // Issue Type and Severity belong to the ordinary case form only. Asserting
    // they are absent is the check that this route really is in security-report
    // mode, rather than the case form at a different URL.
    await expect(form.issueTypeSelect()).toBeHidden();
    await expect(form.severitySelect()).toBeHidden();

    await form.selectDeployment(project.deployment);
    await form.selectProductVersion(project.productVersion);

    // The title is not typed: choosing a deployment and product generates it as
    // "<deployment> - <product name> - YYYY-MM-DD", overwriting anything
    // entered. Asserting it here means the report is submitted with a title we
    // have actually verified rather than an unchecked one.
    await expect(form.titleInput()).toHaveValue(
      SecurityReportCreatePage.expectedTitlePattern(
        project.deployment,
        project.productName,
      ),
    );

    await form.fillDescription(SECURITY_REPORT_INPUT.description);
    await form.attachSecurityReport(SECURITY_REPORT_INPUT.attachmentPath);

    await expect(form.submitButton()).toBeEnabled();

    // Capture the created report from the response so the assertions below
    // prove the backend accepted it, not just that the UI moved on. Security
    // reports post to /cases like every other case type.
    const [createResponse] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes("/cases") &&
          r.request().method() === "POST" &&
          isSuccess(r.status()),
      ),
      form.submit(),
    ]);

    const created = (await createResponse.json()) as {
      id?: string;
      number?: string;
    };
    expect(created.id, "backend returned no case id").toBeTruthy();

    await expect(page).toHaveURL(
      new RegExp(`/projects/${project.id}/.*/${created.id}`),
    );

    console.log(
      `Created security report (${ProjectType.MANAGED_CLOUD_SUBSCRIPTION}): ` +
        `${created.number ?? created.id}`,
    );
  });
});
