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
// How the UI is addressed: accessible names, element ids, and test ids.
// Environment-specific data (project ids, form values) lives in
// `../config/testData.ts`.
//

/** Accessible name of the header control that starts the issue/case flow.
 * Rendered by GetHelpDropdown as a split button: this is the primary half
 * (aria-label="Get Help"), which goes straight to the flow. The other half
 * ("More help options") opens the Issue / Service Request / Security Report
 * menu. */
export const GET_HELP_BUTTON = "Get Help";

/** Case-creation form. Its field labels are sibling <Typography> nodes rather
 * than real <label for>, so `getByLabel` does not work here — the MUI Selects
 * are located by the placeholder text their `renderValue` emits, and the
 * inputs by their stable ids. */
export const CREATE_CASE = {
  heading: "Complete Case Details",
  submitButton: "Create Support Case",
  successMessage: "Case created successfully",
  placeholders: {
    deployment: "Select Deployment...",
    /** Reads "Select deployment first" until a deployment is chosen, and
     * "Select Product..." on Cloud Support projects. */
    productVersion: /Select Product Version|Select Product|Select deployment first/,
  },
  ids: {
    title: "#title",
    issueType: "#issue-type-select",
    severity: "#severity-level-select",
  },
  testIds: {
    /** Lexical rich-text editor — a contenteditable, not an <input>. */
    description: "case-description-editor",
  },
} as const;

/** Get Help dropdown menu items (the arrow half of the split button). */
export const GET_HELP_MENU = {
  trigger: "More help options",
  items: {
    issue: "Issue",
    serviceRequest: "Service Request",
    securityReport: "Security Report",
  },
} as const;

/** Create-service-request form
 * (`/projects/:projectId/support/service-requests/create`).
 *
 * Deployment and Product reuse the same controls as the case form. The Request
 * Details fields below are rendered from the selected catalog item's ServiceNow
 * variables (VariableFormFields.tsx), so they have no stable ids and no
 * `<label for>` — see ServiceRequestCreatePage for how they are addressed. */
export const CREATE_SERVICE_REQUEST = {
  heading: "New Service Request",
  requestTypeHeading: "Select Request Type",
  detailsHeading: "Request Details",
  submitButton: "Create Service Request",
  successMessage: /Service request .*created successfully/,
  /** URL segment a created service request's detail page carries. */
  detailPathSegment: "service-requests",
  testIds: {
    /** The shared rich-text Editor hardcodes this id, so the description field
     * carries the same test id here as on the case form. */
    description: "case-description-editor",
  },
} as const;

/** Case detail page (`/projects/:projectId/support/cases/:caseId`).
 *
 * The state-change buttons come from `getAvailableCaseActions(status)`: an open
 * case offers the "Closed" action, rendered in present tense as "Close" by
 * `toPresentTenseActionLabel`. Clicking it opens a confirmation dialog rather
 * than closing outright. */
export const CASE_DETAIL = {
  /** URL segment every case detail page carries. */
  pathSegment: "support/cases",
  /** The app's <main> region (AppShellLayout). Actions must be scoped to it:
   * the promo banner outside it renders its own dismiss control also named
   * "Close", which otherwise makes the locator ambiguous. */
  mainTestId: "app-main",
  closeButton: "Close",
  /** Status shown in the header chip once the case is closed. */
  closedStatus: "Closed",
  confirmDialog: {
    title: "Confirm State Change",
    confirmButton: "Confirm",
    cancelButton: "Cancel",
  },
} as const;
