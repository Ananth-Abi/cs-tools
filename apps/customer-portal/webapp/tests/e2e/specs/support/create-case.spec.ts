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
// Creates a support case through the header's "Get Help" button: once per
// project type, plus the remaining severity levels on the Subscription project.
// The project, deployment, product version and case content all come from
// ../config/testData.ts — a type whose fixture is not fully filled in skips
// with a message naming the missing fields, rather than failing.
//
// ⚠️ These tests write to a REAL backend. `POST /cases` has no delete
// counterpart, so every test leaves a permanent case on the target environment
// (staging by default). The configured descriptions are deliberately
// self-describing so the records stay identifiable.
//
// The "Get Help" primary button reaches this form directly only when the
// project's Novera AI agent is disabled; with it enabled the same button opens
// the describe-issue chat page instead (see handleIssue in
// GetHelpDropdown.tsx). The page object asserts the resulting URL so that
// divergence surfaces clearly instead of as a missing-field error.
//

import { test, withSession } from "../../fixtures/test";
import {
  CASE_INPUT,
  PROJECTS,
  ProjectType,
  SUBSCRIPTION_SEVERITY_CASES,
} from "../../config/testData";
import {
  createCaseViaGetHelp,
  skipWhenUnconfigured,
} from "../../utils/caseFlows";

withSession(test);

test.describe("Create Case", () => {
  // Each case spans a project-dashboard load, a route change, and four
  // backend-populated dropdowns against a shared staging environment; the
  // 30s default is not enough for the cold first navigation. Set on the suite
  // so every case added here inherits it.
  test.describe.configure({ timeout: 120_000 });

  for (const projectType of Object.values(ProjectType)) {
    const project = PROJECTS[projectType];
    const caseInput = CASE_INPUT[projectType];

    test(`${projectType} — via Get Help`, async ({ page }) => {
      skipWhenUnconfigured(project);
      const created = await createCaseViaGetHelp(page, project, caseInput);
      // Surfaced in the run output so the permanent record this test created is
      // traceable without digging through the HTML report.
      console.log(
        `Created case (${projectType}): ${created.number ?? created.id}`,
      );
    });
  }

  // Severity coverage beyond the S4 default, on the Subscription project only.
  // Which levels a project offers depends on its `acceptedSeverityValues`, so
  // this deliberately targets one project rather than every type.
  const subscription = PROJECTS[ProjectType.SUBSCRIPTION];

  for (const caseInput of SUBSCRIPTION_SEVERITY_CASES) {
    test(`${ProjectType.SUBSCRIPTION} — severity ${caseInput.severity}`, async ({
      page,
    }) => {
      skipWhenUnconfigured(subscription);
      const created = await createCaseViaGetHelp(page, subscription, caseInput);
      console.log(
        `Created case (${ProjectType.SUBSCRIPTION}, ${caseInput.severity}): ` +
          `${created.number ?? created.id}`,
      );
    });
  }
});
