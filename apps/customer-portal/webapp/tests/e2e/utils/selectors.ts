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
  /** Maximum title length enforced by CreateCasePage's handleSubmit and shown
   * by the Title field's counter. */
  titleMaxLength: 160,
  titleCounter: /^\d+\/160$/,
  titleTooLongError: "Title must be 160 characters or fewer.",
  /** Banner messages from `showError` when submit validation rejects. */
  validationErrors: {
    missingTitle: "Please enter a case title.",
    missingDescription: "Please enter a description.",
    missingDeployment: "Please select a deployment type.",
    missingProduct: "Please select a product version.",
  },
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

/** Side navigation. Items are buttons inside the sidebar landmark; which ones
 * render depends on the project's feature flags (see SideBar.tsx). */
export const SIDE_NAV = {
  items: {
    dashboard: "Dashboard",
    support: "Support",
    operations: "Operations",
    updates: "Updates",
    securityCenter: "Security Center",
    engagements: "Engagements",
    usageMetrics: "Usage & Metrics",
    projectDetails: "Project Details",
    announcements: "Announcements",
    settings: "Settings",
  },
} as const;

/** Project details page (`/projects/:projectId/project-details`), reached from
 * the side nav. Its tabs are Overview, Deployments and Time Tracking. */
export const PROJECT_DETAILS = {
  navItem: "Project Details",
  pathSegment: "project-details",
  tabs: {
    overview: "Overview",
    deployments: "Deployments",
    timeTracking: "Time Tracking",
  },
} as const;

/** Overview tab of the project details page.
 *
 * Every entry below is a rendered label rather than a value: the values are
 * environment data (dates, tiers, hours) that change, so specs assert the
 * fields are present and populated rather than pinning them. */
export const PROJECT_OVERVIEW = {
  sections: {
    projectInformation: "Project Information",
    contactInformation: "Contact Information",
    serviceHoursAllocations: "Service Hours Allocations",
  },
  labels: {
    projectName: "Project Name",
    createdDate: "Created Date",
    supportTier: "Support Tier",
    goLiveDate: "Go Live Date",
    subscriptionPeriod: "Subscription Period",
    start: "Start",
    end: "End",
    remaining: "Remaining",
    accountManager: "Account Manager",
    queryHours: "Query Hours",
  },
  /** Rendered in place of a value the API did not return — see the `"--"`
   * fallbacks in ProjectInformationCard. */
  emptyValue: "--",
} as const;

/** Add Deployment modal, opened from the Deployments tab.
 *
 * Unusually for this app the fields have real ids and associated labels, so no
 * structural locators are needed here. */
export const ADD_DEPLOYMENT = {
  openButton: "Add Deployment",
  dialogTitle: "Add New Deployment",
  /** The modal's confirm control carries the same name as the button that opens
   * it, so it must be scoped to the dialog. */
  submitButton: "Add Deployment",
  ids: {
    name: "#deployment-name",
    type: "#deployment-type",
    description: "#deployment-description",
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

/** Create-security-report form
 * (`/projects/:projectId/support/security-report/create`).
 *
 * Rendered by the same CreateCasePage as a normal case, with `isSecurityReport`
 * derived from the path. That flag hides Issue Type and Severity, adds the
 * attachment section, and forces skipChat — so unlike the case form, this route
 * can be opened directly by URL. */
export const CREATE_SECURITY_REPORT = {
  submitButton: "Submit Security Report",
  attachSectionLabel: "Attach Security Report",
  /** The dropzone that opens the upload modal. */
  uploadDropzone: "Upload files",
  /** Shown when submitting with no attachment. */
  missingAttachmentError:
    "Please attach at least one security report file.",
  /** The shared UploadAttachmentModal, opened by the dropzone. Its confirm
   * button reads "Add" (not "Upload") because CreateCasePage passes `onSelect`,
   * so the file is held locally until the report itself is submitted. */
  uploadModal: {
    title: "Upload Attachment",
    confirmButton: "Add",
    nameField: "Attachment name",
  },
} as const;

/** Cases list (`/projects/:projectId/support/cases`). */
export const CASES_LIST = {
  pathSegment: "support/cases",
  /** Default placeholder of ListSearchPanel's input. */
  searchPlaceholder: /Search cases/,
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
  /** URL segment for a security report analysis, which reuses the same header. */
  sraPathSegment: "security-center/security-report-analysis",
  /** URL segment for a service request, which reuses the same header too. */
  serviceRequestPathSegment: "operations/service-requests",
  /** URL segment for an announcement — also the same header. */
  announcementPathSegment: "announcements",
  /** The app's <main> region (AppShellLayout). Actions must be scoped to it:
   * the promo banner outside it renders its own dismiss control also named
   * "Close", which otherwise makes the locator ambiguous. */
  mainTestId: "app-main",
  closeButton: "Close",
  /** Status shown in the header chip once the case is closed. */
  closedStatus: "Closed",
  /** The header row renders, in order: WSO2 case id, case number, a status chip,
   * a severity chip, then the subject. None carry ids or test ids, so they are
   * addressed positionally within the header — see CaseDetailPage. */
  header: {
    /** Case states the header may show, for asserting the value is a real one
     * rather than a placeholder. Mirrors CaseStatus in supportConstants.ts. */
    states: [
      "Open",
      "Work In Progress",
      "Awaiting Info",
      "Waiting On WSO2",
      "Solution Proposed",
      "Reopened",
      "Closed",
    ],
  },
  /** Label on the Details tab. The header shows the same value, so the view spec
   * asserts it there rather than switching tabs. */
  wso2CaseIdLabel: "WSO2 Case ID",
  confirmDialog: {
    title: "Confirm State Change",
    confirmButton: "Confirm",
    cancelButton: "Cancel",
  },
} as const;
