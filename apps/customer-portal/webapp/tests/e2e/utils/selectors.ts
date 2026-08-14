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

/** Security Center (`/projects/:projectId/security-center`), whose default tab is
 * the Security Report Analysis list. */
export const SECURITY_CENTER = {
  pathSegment: "security-center",
  /** The list search covers case number, title AND description — which is what
   * lets a report be found by its stable description rather than by its
   * date-stamped generated title. */
  searchPlaceholder: /Search reports/,
  emptyMessage: "No reports found.",
} as const;

/** The comment box on a case's Activity tab (ActivityCommentInput). */
export const CASE_COMMENT_INPUT = {
  /** Placeholder of the rich-text editor. */
  placeholder: "Write a comment...",
  /** The send control's accessible name — it carries an aria-label as well as a
   * matching tooltip. */
  sendButton: "Send comment",
} as const;

/** The case detail page's Details tab (CaseDetailsDetailsPanel).
 *
 * Every label below is copied verbatim from the component — including
 * "Production Version", which really is spelt that way. The overview ID label is
 * dynamic ("Case ID" for a case, "Service Request Overview" / "Security Report
 * Analysis ID" / "Engagement ID" for the other kinds), so only the case value
 * appears here. */
export const CASE_DETAILS_PANEL = {
  tab: "Details",
  sections: {
    caseOverview: "Case Overview",
    escalationLevels: "Escalation Levels",
    productEnvironment: "Product & Environment",
    customerInformation: "Customer Information",
    watchList: "Watch List",
  },
  fields: {
    caseOverview: [
      "Case ID",
      "WSO2 Case ID",
      "Status",
      "Severity",
      "Category",
      "Created by",
      "Created Date",
      "Last Updated",
    ],
    productEnvironment: ["Product Name", "Production Version"],
    customerInformation: ["Organization", "Project"],
  },
} as const;

/** The Attachments tab of a case (CaseDetailsAttachmentsPanel).
 *
 * Unlike cases, comments and deployments, attachments CAN be deleted — there is a
 * delete affordance per row plus a confirmation dialog — so the attachment spec
 * cleans up after itself instead of accumulating records. */
export const CASE_ATTACHMENTS = {
  /** The tab label carries a live count, e.g. "Attachments (0)". */
  tab: /^Attachments/,
  uploadButton: "Upload Attachment",
  emptyMessage: "No attachments found.",
  uploadModal: {
    title: "Upload Attachment",
    /** Opens the OS file picker via the dropzone's hidden input. */
    chooseFileButton: "Choose file",
    /** Reads "Upload" here — the "Add" variant only appears where the caller
     * passes `onSelect` to hold the file locally, which this panel does not. */
    confirmButton: "Upload",
    nameField: "Attachment name",
  },
  /** Each row reads "<name> | <size> | • | Uploaded by <email> | • | <date>". */
  row: {
    /** formatFileSize output, e.g. "788.2 KB", "30 B", "1.2 MB". */
    sizePattern: /\d+(\.\d+)? (B|KB|MB)/,
    uploadedByPrefix: /Uploaded by \S+/,
    /** formatDateTime output, e.g. "Aug 13, 2026, 9:41 PM". */
    datePattern: /[A-Z][a-z]{2} \d{1,2}, \d{4}, \d{1,2}:\d{2} [AP]M/,
  },
  /** Image rows carry a preview toggle; the label flips once expanded. */
  expandImageButton: "Expand image",
  collapseImageButton: "Collapse image",
  /** The tab label carries the count, e.g. "Attachments (1)". */
  tabCountPattern: /^Attachments \((\d+)\)/,
  deleteModal: {
    title: "Confirm Action",
    confirmButton: "Confirm",
    cancelButton: "Go Back",
  },
} as const;

/** Support Center landing page (`/projects/:projectId/support`), reached from
 * the side nav. Its Outstanding Cases card is the only entry point to the
 * filtered "my cases" list — the URL is otherwise undiscoverable in the UI. */
export const SUPPORT_CENTER = {
  navItem: "Support",
  pathSegment: "support",
  outstandingCases: {
    title: "Outstanding Cases",
    /** Footer buttons of the card, in render order. */
    myCasesButton: "View my cases",
    allCasesButton: "View all cases",
  },
  /** Every list reached from Support Center offers this, because the card sets
   * `returnTo` on navigation. The cases list falls back to a plain "Back" when
   * it is opened any other way, so this label doubles as a check that the
   * card wired `returnTo` up. */
  backButton: "Back to Support Center",
  /**
   * The four stat cards across the top (SUPPORT_STAT_CONFIGS), in render order,
   * each with the list it opens (SupportPage's `handleStatClick`).
   *
   * `label` is the card; `title` is the heading of the page it opens. They are
   * not always the same string — the "Resolved via Chat" card is parenthesised
   * and its destination heading is not.
   *
   * A card navigates whatever its count, including zero, so specs assert the
   * destination page rather than its rows.
   */
  statCards: [
    {
      label: "Outstanding Cases",
      pathSegment: "support/cases",
      query: "statusFilter=active",
      title: "Outstanding Cases",
      description: "Cases that are currently in progress",
    },
    {
      label: "Active Chats",
      pathSegment: "support/conversations",
      query: "statusFilter=active",
      title: "Active Chats",
      description: "Conversations currently in progress",
    },
    {
      label: "Resolved Cases (Last 30d)",
      pathSegment: "support/cases",
      query: "statusFilter=resolved",
      title: "Resolved Cases (Last 30d)",
      description: "Cases that have been resolved during the last 30 days",
    },
    {
      label: "Resolved via Chat (Last 30d)",
      pathSegment: "support/conversations",
      query: "statusFilter=resolvedViaChat",
      title: "Resolved via Chat Last 30d",
      description:
        "Conversations that were resolved via chat during the last 30 days",
    },
  ],
} as const;

/** Cases list (`/projects/:projectId/support/cases`).
 *
 * One page serves several lists: `?createdByMe=true` makes it My Cases, and its
 * heading, description and Created By filter all change with it (AllCasesPage). */
export const CASES_LIST = {
  pathSegment: "support/cases",
  /** Default placeholder of ListSearchPanel's input. */
  searchPlaceholder: /Search cases/,
  /** Query string "View my cases" navigates to. */
  myCasesQuery: "createdByMe=true",
  /** ListPageHeader copy, selected by the query string. */
  myCases: {
    title: "My Cases",
    description: "Manage and track your support cases",
  },
  allCases: {
    title: "All Cases",
    description: "Manage and track all your support cases",
  },
  /** ListResultsBar renders "Showing X of Y cases". */
  resultsCountPattern: /Showing (\d+) of (\d+) cases/,
  /** Prefix ListCard puts before the creator on every row that has one. */
  createdByPrefix: "Created by ",
  /** Opens the filter panel (ListSearchBar). The panel is collapsed by default,
   * so no filter label is in the DOM until this is clicked. Its label becomes
   * "Clear Filters (n)" once any filter is applied — these specs apply none. */
  filtersButton: "Filters",
  /** Label of the Created By filter (deriveFilterLabels("createdBy")), which is
   * withheld on My Cases since the list is already narrowed to one creator. */
  createdByFilterLabel: "Created By",
  /** A filter offered on every cases list, whatever the query string. Used to
   * prove the panel is open, so "Created By is absent" cannot pass merely
   * because nothing is rendered. */
  severityFilterLabel: "Severity",
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
  /** The comment editor's test id. The shared Editor hardcodes this id, so the
   * Activity tab's comment box carries the same one as the case form's
   * description field. */
  commentEditorTestId: "case-description-editor",
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
