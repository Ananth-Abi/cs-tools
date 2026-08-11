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
import {
  ADD_DEPLOYMENT,
  CASE_DETAIL,
  PROJECT_DETAILS,
} from "../utils/selectors";

/** How long to allow for the dashboard, the project-details page and the
 * deployments list to load — each is skeletonised while its queries resolve,
 * well beyond the 5s default expect timeout. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the Deployments tab of the project details page
 * (`/projects/:projectId/project-details`).
 */
export class ProjectDeploymentsPage {
  constructor(private readonly page: Page) {}

  /** The app's <main> region. Scoping matters here: the modal's confirm button
   * shares its accessible name with the button that opens it. */
  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * Navigates from the project dashboard to the Deployments tab via the side
   * nav, then waits for the deployments list.
   *
   * @param projectId - Project whose deployments to open.
   */
  async openDeploymentsTab(projectId: string): Promise<void> {
    await this.page.goto(`/projects/${projectId}/dashboard`);

    const navItem = this.page.getByRole("button", {
      name: PROJECT_DETAILS.navItem,
    });
    await expect(navItem).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await navItem.click();
    await expect(this.page).toHaveURL(
      new RegExp(`/projects/${projectId}/${PROJECT_DETAILS.pathSegment}`),
    );

    await this.deploymentsTab().click();
    // The Add Deployment button only renders once the deployments query resolves
    // (and is withheld entirely for a Restricted project), so waiting on it
    // confirms the tab is ready to act on.
    await expect(this.addDeploymentButton()).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  deploymentsTab(): Locator {
    return this.page.getByRole("tab", {
      name: new RegExp(PROJECT_DETAILS.tabs.deployments),
    });
  }

  /** The button that opens the modal. Scoped to <main> and excluded from the
   * dialog, since the modal's confirm control has the same name. */
  addDeploymentButton(): Locator {
    return this.main().getByRole("button", {
      name: ADD_DEPLOYMENT.openButton,
      exact: true,
    });
  }

  modal(): Locator {
    return this.page.getByRole("dialog");
  }

  nameInput(): Locator {
    return this.modal().locator(ADD_DEPLOYMENT.ids.name);
  }

  typeSelect(): Locator {
    return this.modal().locator(ADD_DEPLOYMENT.ids.type);
  }

  descriptionInput(): Locator {
    return this.modal().locator(ADD_DEPLOYMENT.ids.description);
  }

  /** The modal's confirm control, scoped to the dialog to keep it distinct from
   * the button that opened it. */
  modalSubmitButton(): Locator {
    return this.modal().getByRole("button", {
      name: ADD_DEPLOYMENT.submitButton,
      exact: true,
    });
  }

  modalCancelButton(): Locator {
    return this.modal().getByRole("button", { name: "Cancel", exact: true });
  }

  /** Picks a deployment type without touching the other fields. */
  async selectType(type: string): Promise<void> {
    await this.typeSelect().click();
    await this.page.getByRole("option", { name: type, exact: true }).click();
  }

  async cancel(): Promise<void> {
    await this.modalCancelButton().click();
    await expect(this.modal()).toBeHidden();
  }

  /** Opens the Add Deployment modal and waits for it. */
  async openAddDeploymentModal(): Promise<void> {
    await this.addDeploymentButton().click();
    await expect(this.modal()).toBeVisible();
    await expect(
      this.page.getByText(ADD_DEPLOYMENT.dialogTitle),
    ).toBeVisible();
  }

  /**
   * Fills the modal.
   *
   * @param name - Deployment name.
   * @param type - Deployment type option label.
   * @param description - Description text.
   */
  async fillDeployment(
    name: string,
    type: string,
    description: string,
  ): Promise<void> {
    await this.nameInput().fill(name);

    await this.typeSelect().click();
    await this.page.getByRole("option", { name: type, exact: true }).click();

    await this.descriptionInput().fill(description);
  }

  async submit(): Promise<void> {
    await this.modalSubmitButton().click();
  }

  /** Any element showing a deployment's name — for asserting it appears in the
   * list after creation. */
  deploymentEntry(name: string): Locator {
    return this.main().getByText(name, { exact: false }).first();
  }
}
