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
// The four stat cards across the top of Support Center, and the list each one
// opens. Run against every project type.
//
// Read-only — nothing here creates or changes a record.
//
// Every card navigates whatever its count, zero included (several of these read
// 0 on the test tenant), so the assertions are about the destination page rather
// than what it lists. Each card's list is a filtered view of a route these specs
// already cover unfiltered, so the heading *and* the query string are both
// checked: the heading alone would not distinguish "Outstanding Cases" from any
// other view of the same cases list.
//

import { test, expect, withSession } from "../../fixtures/test";
import { SupportCenterPage } from "../../pages/SupportCenterPage";
import { CASE_LIST_PROJECTS, PROJECTS } from "../../config/testData";
import { SUPPORT_CENTER } from "../../utils/selectors";

withSession(test);

test.describe("Support Center", () => {
  // A dashboard load, a nav navigation, the stats query and then a second page
  // with its own list query — well past the 30s default.
  test.describe.configure({ timeout: 180_000 });

  for (const projectType of CASE_LIST_PROJECTS) {
    const project = PROJECTS[projectType];

    test.describe(projectType, () => {
      test("shows the four overview cards", async ({ page }) => {
        test.skip(
          !project.id,
          `${projectType} needs a project id. ` +
            `Fill it in tests/e2e/config/testData.ts.`,
        );

        const support = new SupportCenterPage(page);
        await support.openViaSideNav(project.id);

        for (const card of SUPPORT_CENTER.statCards) {
          await expect.soft(support.statCard(card.label)).toBeVisible();
        }

        // Exactly four — a fifth card appearing is as much a change as one going
        // missing, and the loop above alone would not notice it. Counted off the
        // page rather than off the list that drove the loop, which would only
        // restate the config.
        await expect(support.statCards()).toHaveCount(4);
      });

      for (const card of SUPPORT_CENTER.statCards) {
        test(`the ${card.label} card opens its list`, async ({ page }) => {
          test.skip(!project.id, `${projectType} needs a project id.`);

          const support = new SupportCenterPage(page);
          await support.openViaSideNav(project.id);

          await support.statCard(card.label).click();

          // The query string is what makes this list the card's list; the
          // heading below would read the same for more than one of these views.
          await expect(page).toHaveURL(
            new RegExp(
              `/projects/${project.id}/${card.pathSegment}\\?${card.query}$`,
            ),
          );

          // Wait for the destination to render before reading its heading. The
          // URL changes first, and until the route swaps the Support Center is
          // still mounted — where this very card's label and the overview card's
          // title are both on screen, so the heading assertion below would
          // resolve to two elements and fail on strict mode instead of retrying.
          await support.waitForList();

          const main = page.getByTestId("app-main");
          await expect(
            main.getByRole("heading", { name: card.title, exact: true }),
          ).toBeVisible();
          await expect(
            main.getByText(card.description, { exact: true }),
          ).toBeVisible();

          // And back, which only offers this label because the card passed a
          // `returnTo` — so the round trip is part of what the card has to get
          // right.
          await support.returnFromList(project.id);
          await expect(support.statCard(card.label)).toBeVisible();
        });
      }
    });
  }
});
