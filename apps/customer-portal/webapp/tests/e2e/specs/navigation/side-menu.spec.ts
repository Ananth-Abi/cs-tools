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
// Side-menu visibility per project.
//
// ✅ READ-ONLY. Nothing is created or modified, so this is safe to run
// repeatedly.
//
// Which items render is driven by the project's **feature flags**, not its type
// label — SideBar.tsx removes Operations without service-request or
// change-request access, Engagements without engagements access, Updates without
// updates access, Security Center without SRA or component analysis, and Usage &
// Metrics unless both the project flag and the portal-wide flag are on. That is
// why expectations live per project in SIDE_NAV_VISIBILITY, verified live, rather
// than being derived from the type.
//
// Both presence and absence are asserted. A hidden item is as much a requirement
// as a visible one: Operations appearing on a project without operations access
// would be a real regression.
//
// The checks are soft (`expect.soft`) so a single run reports every mismatch
// rather than stopping at the first — with nine items, knowing the full picture
// matters more than failing fast. One page load per project keeps it quick.
//

import { test, expect, withSession } from "../../fixtures/test";
import { SideNavPage } from "../../pages/SideNavPage";
import {
  PROJECTS,
  ProjectType,
  SIDE_NAV_VISIBILITY,
} from "../../config/testData";

withSession(test);

test.describe("Side Menu", () => {
  // A cold shell load, whose sidebar waits on the project's features.
  test.describe.configure({ timeout: 120_000 });

  for (const projectType of Object.values(ProjectType)) {
    const expected = SIDE_NAV_VISIBILITY[projectType];
    if (!expected) continue;

    const project = PROJECTS[projectType];

    test.describe(projectType, () => {
      test("shows the expected navigation items", async ({ page }) => {
        test.skip(
          !project.id,
          `${projectType} needs a project id. Fill it in tests/e2e/config/testData.ts.`,
        );

        const nav = new SideNavPage(page);
        await nav.open(project.id);

        for (const [item, shouldBeVisible] of Object.entries(expected)) {
          const locator = nav.item(item);
          if (shouldBeVisible) {
            await expect
              .soft(locator, `"${item}" should be visible`)
              .toBeVisible();
          } else {
            // toHaveCount(0) rather than toBeHidden(): a hidden item is not
            // rendered at all here, and toHaveCount states that directly.
            await expect
              .soft(locator, `"${item}" should not be rendered`)
              .toHaveCount(0);
          }
        }
      });
    });
  }
});
