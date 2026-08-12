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

import { type Locator, type Page, expect } from "../fixtures/test";
import { GET_HELP_BUTTON, SIDE_NAV } from "../utils/selectors";

/** How long to allow for the shell to finish loading. The header and sidebar are
 * skeletonised until the projects list resolves, well beyond the 5s default. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the side navigation.
 *
 * Which items render is driven by the project's feature flags, so this exists to
 * assert presence and absence rather than to navigate.
 */
export class SideNavPage {
  constructor(private readonly page: Page) {}

  /**
   * Opens a project and waits for the shell to finish rendering.
   *
   * Waits on the header's Get Help button rather than a nav item: every nav item
   * is exactly what is under test here, so gating readiness on one would make
   * the test unable to distinguish "not rendered yet" from "not rendered at all".
   *
   * @param projectId - Project whose navigation to inspect.
   */
  async open(projectId: string): Promise<void> {
    await this.page.goto(`/projects/${projectId}/dashboard`);
    await expect(
      this.page.getByRole("button", { name: GET_HELP_BUTTON, exact: true }),
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    // The sidebar mounts with the shell, but its items arrive with the project's
    // features; Dashboard is present for every project, so it marks the point
    // where the item list is real rather than still empty.
    await expect(this.item(SIDE_NAV.items.dashboard)).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * The sidebar landmark.
   *
   * The page exposes two `complementary` regions, so this picks the one holding
   * the Dashboard item rather than relying on document order.
   */
  sidebar(): Locator {
    return this.page.getByRole("complementary").filter({
      has: this.page.getByRole("button", {
        name: SIDE_NAV.items.dashboard,
        exact: true,
      }),
    });
  }

  /**
   * A navigation item, scoped to the sidebar so header controls with similar
   * names cannot satisfy it.
   *
   * @param name - Exact item label.
   * @returns Locator for the item.
   */
  item(name: string): Locator {
    return this.sidebar().getByRole("button", { name, exact: true });
  }
}
