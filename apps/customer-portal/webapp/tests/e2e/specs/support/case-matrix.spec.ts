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
// Ensures one case exists per project type per severity — S1 to S4 across
// Subscription, Managed Cloud Subscription and Cloud Support.
//
// ⚠️ IDEMPOTENT BY DESIGN, which matters because cases cannot be deleted. Each
// test searches the project's case list for its deterministic subject
// (`<prefix> <severity code>`, from CASE_MATRIX) and creates a case only when
// none is found. A second run therefore creates nothing.
//
// Two consequences worth understanding before changing anything here:
//
// - The subject IS the key. Renaming a prefix in CASE_MATRIX orphans the
//   existing cases, and the next run recreates that whole row — permanently.
// - A false negative on the existence check creates a duplicate that cannot be
//   removed, so `hasCaseWithSubject` waits for the search response produced by
//   its own query rather than sampling the list mid-flight.
//
// ⚠️ DO NOT RUN THIS SUITE CONCURRENTLY AGAINST ONE ENVIRONMENT. The lookup and
// the create are separate steps, so two overlapping runs can both find nothing
// and both create — leaving a duplicate that cannot be deleted. Within a single
// run this cannot happen (playwright.config.ts pins `workers: 1` and
// `fullyParallel: false`), but nothing stops a second run, on another machine or
// in CI, from racing this one.
//
// This is deliberately a documented constraint rather than a coded guard:
// `POST /cases` offers no idempotency key and no uniqueness on subject, so the
// server cannot dedupe; and a lock file would only serialize runs on the same
// machine, giving false assurance against exactly the cross-machine case that
// matters. Fixing it properly needs a server-side unique-create.
//
// Severity availability is per project: it comes from `acceptedSeverityValues`,
// so a project that does not offer a severity skips that combination rather than
// failing.
//

import { test, expect, withSession } from "../../fixtures/test";
import { CasesListPage } from "../../pages/CasesListPage";
import { CaseCreatePage } from "../../pages/CaseCreatePage";
import {
  CASE_MATRIX,
  CASE_MATRIX_SEVERITIES,
  IssueType,
  PROJECTS,
  ProjectType,
  SEVERITY_CODES,
} from "../../config/testData";
import { expectSuccess, skipWhenUnconfigured } from "../../utils/caseFlows";
import { CREATE_CASE } from "../../utils/selectors";

withSession(test);

test.describe("Case Matrix", () => {
  // Each test may load a list, search it, then run the whole create-case flow;
  // the 30s default is nowhere near enough.
  test.describe.configure({ timeout: 180_000 });

  for (const projectType of Object.values(ProjectType)) {
    const project = PROJECTS[projectType];
    const naming = CASE_MATRIX[projectType];

    test.describe(projectType, () => {
      for (const severity of CASE_MATRIX_SEVERITIES) {
        const code = SEVERITY_CODES[severity];
        const subject = `${naming.titlePrefix} ${code}`;

        test(`has a ${code} case`, async ({ page }) => {
          skipWhenUnconfigured(project);

          const list = new CasesListPage(page);
          await list.open(project.id);

          if (await list.hasCaseWithSubject(subject)) {
            // Nothing to do — this is the steady state after the first run.
            console.log(`${projectType} ${code}: exists ("${subject}")`);
            return;
          }

          console.log(`${projectType} ${code}: missing, creating "${subject}"`);

          const form = new CaseCreatePage(page);
          await form.openViaGetHelp(project.id);

          if (project.autoSelectsDeployment) {
            await expect(form.deploymentSelect()).toBeHidden();
          } else {
            await form.selectDeployment(project.deployment);
          }
          await form.selectProductVersion(project.productVersion);

          // Skip rather than fail when the project does not offer this severity:
          // the available set is project data, not a defect.
          await form.severitySelect().click();
          const option = page.getByRole("option", {
            name: severity,
            exact: true,
          });
          const offered = (await option.count()) > 0;
          if (!offered) {
            await page.keyboard.press("Escape");
          }
          test.skip(
            !offered,
            `${projectType} does not offer ${severity} — its acceptedSeverityValues exclude it.`,
          );
          await option.click();

          await form.fillTitle(subject);
          await form.fillDescription(`${naming.descriptionPrefix} ${code}`);
          await form.selectIssueType(IssueType.QUESTION);

          await expect(form.submitButton()).toBeEnabled();

          const [response] = await Promise.all([
            page.waitForResponse(
              (r) =>
                new URL(r.url()).pathname.endsWith("/cases") &&
                r.request().method() === "POST",
            ),
            form.submit(),
          ]);

          // Status asserted here rather than in the predicate, so a rejected
          // create reports the server's message instead of timing out.
          await expectSuccess(response, "create case");

          const created = (await response.json()) as {
            id?: string;
            number?: string;
          };
          expect(created.id, "backend returned no case id").toBeTruthy();

          await expect(
            page.getByText(CREATE_CASE.successMessage),
          ).toBeVisible();

          console.log(
            `${projectType} ${code}: created ${created.number ?? created.id}`,
          );
        });
      }
    });
  }
});
