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
// Deployment flows shared by more than one spec.
//
// Both the deployment suite and the deployment-product suite start from a
// deployment of their own — each test creates one so that it can run alone —
// so the creation lives here rather than in either spec.
//

import { expect, type Page } from "../fixtures/test";
import { ProjectDeploymentsPage } from "../pages/ProjectDeploymentsPage";
import { DEPLOYMENT_INPUT, type ProjectFixture } from "../config/testData";
import { expectSuccess } from "./caseFlows";

/** How long to allow for the create request and the list to catch up.
 *
 * The suite configures no `expect` timeout, so assertions default to 5s and
 * `waitForResponse` to 30s — both thin for a create against a real backend
 * followed by a list refetch, inside tests that are given 180s. Matches the
 * page objects' own LOAD_TIMEOUT_MS. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Creates a deployment on a project and returns its name.
 *
 * ⚠️ Writes to a REAL backend. The record can be deactivated from the UI but
 * there is no DELETE verb, so a caller that does not delete it leaves it behind.
 *
 * @param page - Test page.
 * @param project - Project to create the deployment on.
 * @returns The page object and the deployment's name.
 */
export async function createDeployment(
  page: Page,
  project: ProjectFixture,
): Promise<{
  deployments: ProjectDeploymentsPage;
  deploymentName: string;
  deploymentId: string;
}> {
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
      { timeout: LOAD_TIMEOUT_MS },
    ),
    deployments.submit(),
  ]);

  await expectSuccess(createResponse, "create deployment");

  const created = (await createResponse.json()) as {
    id?: string;
    name?: string;
  };
  expect(created.id, "backend returned no deployment id").toBeTruthy();

  // The modal closes and the new deployment shows up in the list — exactly
  // once, since the name is unique to this run. Both wait on a refetch, so
  // neither can rely on the 5s default.
  await expect(deployments.modal()).toBeHidden({ timeout: LOAD_TIMEOUT_MS });
  const entry = deployments.deploymentEntry(deploymentName);
  await expect(entry).toHaveCount(1, { timeout: LOAD_TIMEOUT_MS });
  await expect(entry).toBeVisible({ timeout: LOAD_TIMEOUT_MS });

  console.log(
    `Created deployment (${project.type}): ` +
      `${created.name ?? deploymentName} (${created.id})`,
  );

  return {
    deployments,
    deploymentName,
    deploymentId: created.id as string,
  };
}
