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
// Edge cases and field gating on the create-security-report form.
//
// ✅ NOTHING HERE CREATES A RECORD. Every scenario is read-only or is rejected
// by `handleSubmit` before a request is made. Note the rejection order in
// CreateCasePage.tsx — title, then title length, then description, then the PII
// guard, then the security-report attachment check, then deployment, then
// product — which is why the later cases have to fill the earlier fields to
// reach the rule under test.
//
// The Title is not typed on this form: choosing a deployment and a product
// generates it as "<deployment> - <product name> - YYYY-MM-DD", overwriting
// anything entered. Two consequences for the cases below — there is no
// empty-title or over-long-title scenario a user can reach, and the
// "missing product" branch is unreachable, because without a product the title
// is still empty and that check fires first.
//
// Scoped to Managed Cloud Subscription: the Security Report entry point is gated
// on the project's SRA write access (`isSecurityReportVisible` in
// GetHelpDropdown.tsx), so it is not available for every project type.
//
// Each test asserts the form stayed put, so a validation rule that silently
// stopped working surfaces as an unexpected navigation rather than as a passing
// test plus a stray record in staging.
//

import { test, expect, withSession, type Page } from "../../fixtures/test";
import { SecurityReportCreatePage } from "../../pages/SecurityReportCreatePage";
import {
  PROJECTS,
  ProjectType,
  SECURITY_REPORT_INPUT,
} from "../../config/testData";
import { skipWhenUnconfigured } from "../../utils/caseFlows";
import { CREATE_CASE, CREATE_SECURITY_REPORT } from "../../utils/selectors";

withSession(test);

test.describe("Security Report — validation", () => {
  // Each test loads the dashboard and then the form, both of which are
  // skeletonised while their queries resolve; the 30s default is not enough.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[ProjectType.MANAGED_CLOUD_SUBSCRIPTION];
  const input = SECURITY_REPORT_INPUT[ProjectType.MANAGED_CLOUD_SUBSCRIPTION];

  /** Opens the form ready for a validation assertion. */
  async function openForm(page: Page): Promise<SecurityReportCreatePage> {
    const form = new SecurityReportCreatePage(page);
    await form.openViaGetHelpMenu(project.id);
    return form;
  }

  test("hides issue type and severity", async ({ page }) => {
    skipWhenUnconfigured(project);
    const form = await openForm(page);

    // These belong to the ordinary case form only. Their absence is what
    // distinguishes security-report mode from the case form at another URL.
    await expect(form.issueTypeSelect()).toBeHidden();
    await expect(form.severitySelect()).toBeHidden();
  });

  test("keeps submit disabled until a deployment is chosen", async ({ page }) => {
    skipWhenUnconfigured(project);
    const form = await openForm(page);

    await expect(form.submitButton()).toBeDisabled();
    await form.selectDeployment(project.deployment);
    await expect(form.submitButton()).toBeEnabled();
  });

  test("gates product selection on choosing a deployment", async ({ page }) => {
    skipWhenUnconfigured(project);
    const form = await openForm(page);

    await expect(form.productVersionSelect()).toBeDisabled();
    await form.selectDeployment(project.deployment);
    await expect(form.productVersionSelect()).toBeEnabled();
  });

  test("generates the title from deployment, product and today's date", async ({
    page,
  }) => {
    skipWhenUnconfigured(project);
    const form = await openForm(page);

    // Empty until BOTH are chosen — the auto-fill effect bails out unless it has
    // a deployment and a product to build the title from.
    await expect(form.titleInput()).toHaveValue("");
    await form.selectDeployment(project.deployment);
    await expect(form.titleInput()).toHaveValue("");

    await form.selectProductVersion(project.productVersion);
    await expect(form.titleInput()).toHaveValue(
      SecurityReportCreatePage.expectedTitlePattern(
        project.deployment,
        project.productName,
      ),
    );
  });

  test("rejects submit before a product is chosen", async ({ page }) => {
    skipWhenUnconfigured(project);
    const form = await openForm(page);
    await form.selectDeployment(project.deployment);
    await form.fillDescription(input.description);

    // Submit unlocks as soon as a deployment is set, but the title is only
    // generated once a product is too — so the first rule handleSubmit hits is
    // the empty title, not the missing product. Worth pinning down: the message
    // names a field the user cannot fill, which reads as misleading.
    await form.attemptSubmit();

    await expect(form.errorAlert()).toContainText(
      CREATE_CASE.validationErrors.missingTitle,
    );
    await expect(page).toHaveURL(/security-report\/create/);
  });

  test("rejects submit when the description is empty", async ({ page }) => {
    skipWhenUnconfigured(project);
    const form = await openForm(page);
    await form.selectDeployment(project.deployment);
    await form.selectProductVersion(project.productVersion);
    // Title is generated by the two selections above, so description is the only
    // thing missing.

    await form.attemptSubmit();

    await expect(form.errorAlert()).toContainText(
      CREATE_CASE.validationErrors.missingDescription,
    );
    await expect(page).toHaveURL(/security-report\/create/);
  });

  test("requires at least one attachment", async ({ page }) => {
    skipWhenUnconfigured(project);
    const form = await openForm(page);
    await form.selectDeployment(project.deployment);
    await form.selectProductVersion(project.productVersion);
    await form.fillDescription(input.description);

    // The rule that only exists for security reports. Everything else on the
    // form is valid — title included, since the selections generated it — so
    // this isolates the attachment requirement.
    await form.attemptSubmit();

    await expect(form.errorAlert()).toContainText(
      CREATE_SECURITY_REPORT.missingAttachmentError,
    );
    await expect(page).toHaveURL(/security-report\/create/);
  });

  test("keeps the upload modal's Add disabled until a file is chosen", async ({
    page,
  }) => {
    skipWhenUnconfigured(project);
    const form = await openForm(page);

    await form.openUploadModal();
    await expect(form.uploadModalConfirm()).toBeDisabled();
    await form.cancelUploadModal();
  });
});
