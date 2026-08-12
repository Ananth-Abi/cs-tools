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
// Edge cases and field gating on the Add Deployment modal.
//
// ✅ NOTHING HERE CREATES A DEPLOYMENT. The required-field cases never submit —
// AddDeploymentModal gates its confirm button on
// `name && deploymentTypeKey && description`, so an incomplete form cannot be
// sent at all. The duplicate-name case does submit, but the backend rejects it
// with 409, and the cancel case closes the modal without submitting.
//
// That matters more here than elsewhere: deployments have NO delete endpoint, so
// anything this suite created would be permanent and unremovable.
//

import { test, expect, withSession } from "../../fixtures/test";
import { ProjectDeploymentsPage } from "../../pages/ProjectDeploymentsPage";
import {
  DEPLOYMENT_INPUT,
  PROJECTS,
  ProjectType,
} from "../../config/testData";
import { skipWhenUnconfigured } from "../../utils/caseFlows";

withSession(test);

test.describe("Deployment — validation", () => {
  // Each test loads the dashboard, navigates the side nav and switches tab;
  // the 30s default is not enough.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[ProjectType.MANAGED_CLOUD_SUBSCRIPTION];

  /** Path of the create endpoint, distinct from the /deployments/search POST
   * that also fires on this page. */
  const createPath = `/projects/${project.id}/deployments`;

  test("keeps submit disabled until every required field is filled", async ({
    page,
  }) => {
    skipWhenUnconfigured(project);
    const deployments = new ProjectDeploymentsPage(page);
    await deployments.openDeploymentsTab(project.id);
    await deployments.openAddDeploymentModal();

    // isValid requires all three; check each step rather than only the endpoint,
    // so a rule that stopped covering one field would still fail here.
    await expect(deployments.modalSubmitButton()).toBeDisabled();

    await deployments.nameInput().fill(DEPLOYMENT_INPUT.namePrefix);
    await expect(deployments.modalSubmitButton()).toBeDisabled();

    await deployments.selectType(DEPLOYMENT_INPUT.type);
    await expect(deployments.modalSubmitButton()).toBeDisabled();

    await deployments.descriptionInput().fill(DEPLOYMENT_INPUT.description);
    await expect(deployments.modalSubmitButton()).toBeEnabled();
  });

  test("keeps submit disabled when only the description is missing", async ({
    page,
  }) => {
    skipWhenUnconfigured(project);
    const deployments = new ProjectDeploymentsPage(page);
    await deployments.openDeploymentsTab(project.id);
    await deployments.openAddDeploymentModal();

    await deployments.nameInput().fill(DEPLOYMENT_INPUT.namePrefix);
    await deployments.selectType(DEPLOYMENT_INPUT.type);

    await expect(deployments.modalSubmitButton()).toBeDisabled();
  });

  test("keeps submit disabled for a whitespace-only name", async ({ page }) => {
    skipWhenUnconfigured(project);
    const deployments = new ProjectDeploymentsPage(page);
    await deployments.openDeploymentsTab(project.id);
    await deployments.openAddDeploymentModal();

    // isValid trims before comparing, so spaces must not count as a name.
    await deployments.nameInput().fill("   ");
    await deployments.selectType(DEPLOYMENT_INPUT.type);
    await deployments.descriptionInput().fill(DEPLOYMENT_INPUT.description);

    await expect(deployments.modalSubmitButton()).toBeDisabled();
  });

  test("rejects a duplicate deployment name", async ({ page }) => {
    skipWhenUnconfigured(project);
    const deployments = new ProjectDeploymentsPage(page);
    await deployments.openDeploymentsTab(project.id);
    await deployments.openAddDeploymentModal();

    await deployments.fillDeployment(
      DEPLOYMENT_INPUT.existingName,
      DEPLOYMENT_INPUT.type,
      DEPLOYMENT_INPUT.description,
    );

    const [response] = await Promise.all([
      page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname.endsWith(createPath) &&
          r.request().method() === "POST",
      ),
      deployments.submit(),
    ]);

    // The backend enforces name uniqueness per project. Nothing is created, so
    // this case is safe to re-run.
    expect(response.status()).toBe(409);
    expect(await response.text()).toContain("already exists");

    // The modal stays open on failure. Note the UI surfaces no visible error
    // here — from the user's side the button simply does nothing — which is
    // worth treating as a product defect rather than expected behaviour.
    await expect(deployments.modal()).toBeVisible();
  });

  test("cancelling closes the modal without creating a deployment", async ({
    page,
  }) => {
    skipWhenUnconfigured(project);
    const deployments = new ProjectDeploymentsPage(page);
    await deployments.openDeploymentsTab(project.id);
    await deployments.openAddDeploymentModal();

    let createAttempts = 0;
    page.on("request", (r) => {
      if (
        new URL(r.url()).pathname.endsWith(createPath) &&
        r.method() === "POST"
      ) {
        createAttempts += 1;
      }
    });

    await deployments.fillDeployment(
      `${DEPLOYMENT_INPUT.namePrefix} cancelled`,
      DEPLOYMENT_INPUT.type,
      DEPLOYMENT_INPUT.description,
    );
    await deployments.cancel();

    expect(createAttempts, "cancel must not send a create request").toBe(0);
  });
});
