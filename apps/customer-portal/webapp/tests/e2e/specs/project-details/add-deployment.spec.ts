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
// Adds a deployment from the Deployments tab of the project details page,
// reached through the side nav as a user would.
//
// ⚠️ Writes to a REAL backend via POST /projects/{id}/deployments and leaves a
// permanent deployment on every run — there is no delete endpoint.
//
// The name carries a timestamp because the backend rejects a duplicate with
// `409 {"message":"A deployment with the same name already exists for the
// project."}` (confirmed live). A fixed name therefore only ever works on the
// first run; every later one failed, and — because the response wait originally
// required a 2xx — failed as an unexplained 180s timeout rather than as a 409.
//
// Scoped to Managed Cloud Subscription: the Add Deployment button is withheld
// for a Restricted project, and deployment access is a per-project feature flag.
//

import { test, expect, withSession } from "../../fixtures/test";
import { ProjectDeploymentsPage } from "../../pages/ProjectDeploymentsPage";
import {
  DEPLOYMENT_INPUT,
  PROJECTS,
  ProjectType,
} from "../../config/testData";
import { isSuccess, skipWhenUnconfigured } from "../../utils/caseFlows";

withSession(test);

test.describe("Deployment", () => {
  // Spans a dashboard load, a side-nav navigation, a tab switch and a
  // filters-backed dropdown; the 30s default is not enough.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[ProjectType.MANAGED_CLOUD_SUBSCRIPTION];

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — adds a deployment`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    const deployments = new ProjectDeploymentsPage(page);

    // Unique per run, so the 409-on-duplicate rule cannot fail the test. Uses a
    // sortable compact timestamp to keep the accumulated records readable.
    // Milliseconds are kept: at whole-second resolution two runs starting in the
    // same second — a retry, or a colleague running against the same staging
    // project — would collide and hit the very 409 this is here to avoid.
    const deploymentName = `${DEPLOYMENT_INPUT.namePrefix} ${new Date()
      .toISOString()
      .slice(0, 23)
      .replace(/[:T.]/g, "-")}`;

    await deployments.openDeploymentsTab(project.id);
    await deployments.openAddDeploymentModal();
    await deployments.fillDeployment(
      deploymentName,
      DEPLOYMENT_INPUT.type,
      DEPLOYMENT_INPUT.description,
    );

    await expect(deployments.modalSubmitButton()).toBeEnabled();

    // Wait for the create POST regardless of status, then assert it succeeded.
    // Requiring 2xx in the predicate instead would mean a rejected create never
    // matches, leaving the test to time out with no clue as to why — so the
    // status check belongs in an assertion that can report the body.
    //
    // The path must match exactly: POST /projects/{id}/deployments/search fires
    // on this page too, and a substring match would happily capture it.
    const [createResponse] = await Promise.all([
      page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname.endsWith(
            `/projects/${project.id}/deployments`,
          ) && r.request().method() === "POST",
      ),
      deployments.submit(),
    ]);

    expect(
      isSuccess(createResponse.status()),
      `create deployment failed: ${createResponse.status()} ${await createResponse.text()}`,
    ).toBe(true);

    const created = (await createResponse.json()) as {
      id?: string;
      name?: string;
    };
    expect(created.id, "backend returned no deployment id").toBeTruthy();

    // The modal closes and the new deployment shows up in the list — exactly
    // once, since the name is unique to this run.
    await expect(deployments.modal()).toBeHidden();
    const entry = deployments.deploymentEntry(deploymentName);
    await expect(entry).toHaveCount(1);
    await expect(entry).toBeVisible();

    console.log(
      `Created deployment (${ProjectType.MANAGED_CLOUD_SUBSCRIPTION}): ` +
        `${created.name ?? deploymentName} (${created.id})`,
    );
  });
});
