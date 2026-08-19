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
// Raises a service request from the "Get Help" split button's dropdown.
//
// Scoped to the Managed Cloud Subscription project: the Service Request menu
// item only appears when the project has service-request access
// (`isServiceRequestVisible` in GetHelpDropdown.tsx), which is a per-project
// feature flag rather than something every type has.
//
// ⚠️ Writes to a REAL backend and leaves a permanent service request on every
// run — there is no delete counterpart. The configured description is
// deliberately self-describing so the records stay identifiable.
//

import { test, expect, withSession } from "../../fixtures/test";
import { ServiceRequestCreatePage } from "../../pages/ServiceRequestCreatePage";
import {
  PROJECTS,
  ProjectType,
  SERVICE_REQUEST_INPUT,
} from "../../config/testData";
import { isSuccess, skipWhenUnconfigured } from "../../utils/caseFlows";
import { CREATE_SERVICE_REQUEST } from "../../utils/selectors";

withSession(test);

test.describe("Service Request", () => {
  // Spans a dashboard load, a menu navigation, two backend-populated dropdowns,
  // a catalog fetch and a variables fetch; the 30s default is not enough.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[ProjectType.MANAGED_CLOUD_SUBSCRIPTION];

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — creates a generic request`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    const serviceRequest = new ServiceRequestCreatePage(page);

    await serviceRequest.openViaGetHelpMenu(project.id);

    await serviceRequest.selectDeployment(project.deployment);
    await serviceRequest.selectProductVersion(project.productVersion);
    await serviceRequest.selectRequestType(
      SERVICE_REQUEST_INPUT.catalog,
      SERVICE_REQUEST_INPUT.catalogItem,
    );
    await serviceRequest.fillRequestDetails(SERVICE_REQUEST_INPUT.requestDetails);
    await serviceRequest.fillDescription(SERVICE_REQUEST_INPUT.description);

    await expect(serviceRequest.submitButton()).toBeEnabled();

    // Capture the created request from the response so the assertions below
    // prove the backend accepted it, not just that the UI moved on.
    //
    // Note the endpoint is POST /cases, not /service-requests: a service
    // request is a case carrying `requestType: "service_request"` (see
    // usePostCase.ts). Only the detail URL it redirects to is SR-specific.
    const [createResponse] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes("/cases") &&
          r.request().method() === "POST" &&
          isSuccess(r.status()),
      ),
      serviceRequest.submit(),
    ]);

    const created = (await createResponse.json()) as {
      id?: string;
      number?: string;
    };
    expect(created.id, "backend returned no service request id").toBeTruthy();

    // On success the page routes to the new request's detail page and shows a
    // banner naming it.
    await expect(page).toHaveURL(
      new RegExp(
        `/projects/${project.id}/.*${CREATE_SERVICE_REQUEST.detailPathSegment}/${created.id}`,
      ),
    );
    await expect(
      page.getByText(CREATE_SERVICE_REQUEST.successMessage),
    ).toBeVisible();

    console.log(
      `Created service request (${ProjectType.MANAGED_CLOUD_SUBSCRIPTION}): ` +
        `${created.number ?? created.id}`,
    );
  });
});
