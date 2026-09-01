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
// Posting a comment on a case's Activity tab.
//
// ⚠️ IDEMPOTENT, because comments cannot be deleted — `POST /cases/{id}/comments`
// has no delete counterpart. The test posts only when its exact text is not
// already on the case, so retries and repeated runs add nothing. The first run
// exercises the full post path; later runs assert the comment is still there.
//
// The text is the idempotency key: changing it in CASE_COMMENTS makes the next
// run post a new comment on every project, permanently.
//
// Each project comments on its own S1 case — the same records the view-case suite
// asserts on. Those assertions match comments by text, so extra comments do not
// disturb them.
//
// The Activity tab is the case detail page's default tab, so no tab switch is
// needed. The comment box is the shared rich-text Editor — a contenteditable, so
// the text is typed rather than filled, and the send control only enables once
// Lexical registers submittable content.
//

import { test, expect, withSession } from "../../fixtures/test";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import { CASE_COMMENTS, PROJECTS } from "../../config/testData";
import { expectSuccess } from "../../utils/caseFlows";

withSession(test);

test.describe("Case Comment", () => {
  // A cold case load plus a round trip for the post; the 30s default is not
  // enough.
  test.describe.configure({ timeout: 120_000 });

  for (const { projectType, caseId, text } of CASE_COMMENTS) {
    const project = PROJECTS[projectType];

    test.describe(projectType, () => {
      test("adds a comment to the case activity", async ({ page }) => {
        test.skip(
          !project.id || !caseId,
          `${projectType} needs a project id and a case id. ` +
            `Fill them in tests/e2e/config/testData.ts.`,
        );

        const caseDetail = new CaseDetailPage(page);
        await caseDetail.open(project.id, caseId);

        const existing = caseDetail.comment(text);
        if ((await existing.count()) > 0) {
          // Already posted by an earlier run. Assert it is still displayed
          // rather than posting a duplicate that could not be removed.
          console.log(`${projectType}: comment already present`);
          await expect(existing).toBeVisible();
          return;
        }

        console.log(`${projectType}: comment missing, posting it`);

        const response = await caseDetail.addComment(text);

        // Status asserted here rather than in the response predicate, so a
        // rejected post reports the server's message instead of timing out.
        await expectSuccess(response, "post comment");

        // The list refetches after a successful post, so the comment should
        // appear in the activity feed without a reload.
        await expect(caseDetail.comment(text)).toBeVisible();
      });
    });
  }
});
