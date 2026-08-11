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
// MASTER TEST DATA for the E2E suite — VALUES ONLY.
//
// Every spec sources its environment-specific data from here, not just
// case creation. This file holds no locators, no helpers, and no page logic:
// selectors live in `../utils/selectors.ts`, actions in `../pages/`.
//
// Pointing the suite at a different tenant means editing this file and nothing
// else. Values below were captured against **staging**
// (https://support-stg.wso2.com), which is what `.env.e2e` targets.
//

// ─────────────────────────────────────────────────────────────────────────────
// Project fixtures
// ─────────────────────────────────────────────────────────────────────────────

/** Project types this suite exercises.
 *
 * The portal derives feature visibility from `GET /projects/{id}/features`
 * rather than from these labels, but the type still drives a few rules —
 * deployment filtering, case/SR product category, and which severities are on
 * offer — so each type needs its own dedicated project. */
export const ProjectType = {
  SUBSCRIPTION: "Subscription",
  MANAGED_CLOUD_SUBSCRIPTION: "Managed Cloud Subscription",
  CLOUD_SUPPORT: "Cloud Support",
} as const;

export type ProjectType = (typeof ProjectType)[keyof typeof ProjectType];

/** A project the suite runs against, plus the option labels its forms offer.
 *
 * `deployment` and `productVersion` are per-project rather than shared: those
 * dropdowns are populated from the project's own deployments, so the labels
 * differ between projects even for the same project type. */
export interface ProjectFixture {
  /** Project id as it appears in the URL (`/projects/<id>/...`).
   *
   * An empty string means "not yet captured for this environment" — specs must
   * skip rather than run against the wrong project. */
  id: string;
  /** Project name exactly as displayed in the portal. */
  name: string;
  /** The project's type label, for specs that assert type-driven behaviour. */
  type: ProjectType;
  /** Deployment option label, exactly as rendered in the Deployment dropdown.
   * Empty when `autoSelectsDeployment` is true — there is nothing to pick. */
  deployment: string;
  /** Sysid of the deployment above. The case form selects by label, so this is
   * only needed by specs that assert against the API payload or filter by
   * deployment in a URL. */
  deploymentId: string;
  /** True when the case form hides the Deployment field and locks it to the
   * project's primary production deployment. Cloud Support and Cloud Evaluation
   * Support behave this way (`shouldRestrictToPrimaryProductionDeployments` →
   * `hideDeploymentField` in CreateCasePage.tsx), so specs must not try to pick
   * a deployment for them. */
  autoSelectsDeployment: boolean;
  /** Product option label, exactly as rendered. The field itself is labelled
   * "Product Version" on most types but "Product" on Cloud Support. */
  productVersion: string;
  /** The same product's name *without* its version — i.e. `product.label` as
   * the API returns it, not the dropdown text.
   *
   * Needed because the security-report form builds its title from this rather
   * than from the option label (see the auto-fill effect in
   * CreateCasePage.tsx). Cannot be derived from `productVersion` by trimming, so
   * it is recorded explicitly. */
  productName: string;
}

/**
 * The dedicated automation projects, one per project type.
 *
 * `deployment` and `productVersion` still need capturing for every entry — take
 * them from each project's own case form, since the options come from that
 * project's deployments. Specs that need them must skip while they are empty.
 */
export const PROJECTS: Record<ProjectType, ProjectFixture> = {
  [ProjectType.SUBSCRIPTION]: {
    id: "641058e63b5a87103e1e088aa4e45a13",
    name: "Automation Test Customer Project - Subscription",
    type: ProjectType.SUBSCRIPTION,
    deployment: "Production",
    deploymentId: "8f8a33693bee8b503e1e088aa4e45ab4",
    autoSelectsDeployment: false,
    productVersion: "WSO2 API Manager 4.5.0",
    productName: "WSO2 API Manager",
  },
  [ProjectType.MANAGED_CLOUD_SUBSCRIPTION]: {
    id: "a0873629eba28f90fcf5f5dabad0cda0",
    name: "Automation Test MS Customer Project - Managed Cloud Subscription",
    type: ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
    deployment: "Production",
    deploymentId: "f40cf7e53b2a4b9091404c6aa5e45a00",
    autoSelectsDeployment: false,
    productVersion: "WSO2 Identity Server 7.1.0",
    productName: "WSO2 Identity Server",
  },
  [ProjectType.CLOUD_SUPPORT]: {
    id: "cd9776ed3ba28b503e1e088aa4e45a81",
    name: "Automation Test Cloud Customer Project - Cloud Support",
    type: ProjectType.CLOUD_SUPPORT,
    // Deployment is hidden and auto-locked to primary production for this type.
    deployment: "",
    deploymentId: "",
    autoSelectsDeployment: true,
    productVersion: "WSO2 Developer Platform",
    productName: "WSO2 Developer Platform",
  },
};

// ─────────────────────────────────────────────────────────────────────────────
// Form option vocabularies
// ─────────────────────────────────────────────────────────────────────────────

/** Case severity levels as rendered in the Severity Level dropdown. S4 carries
 * no space before the parenthesis — it is exactly "S4(Query)" (see
 * `CaseSeverityLevel` in src/features/support/constants/supportConstants.ts).
 *
 * Which of these appear depends on the project's `acceptedSeverityValues`:
 * S0 shows only when the project accepts P0, and severity locks to S4 when P4
 * is the only accepted value — so the available set differs by project. */
export const Severity = {
  S0: "S0",
  S1: "S1",
  S2: "S2",
  S3: "S3",
  S4: "S4(Query)",
} as const;

export type Severity = (typeof Severity)[keyof typeof Severity];

/** Issue Type options offered on the case form. */
export const IssueType = {
  TOTAL_OUTAGE: "Total Outage",
  QUESTION: "Question",
  PERFORMANCE_DEGRADATION: "Performance Degradation",
  PARTIAL_OUTAGE: "Partial Outage",
  ERROR: "Error",
  SECURITY_OR_COMPLIANCE: "Security or Compliance",
} as const;

export type IssueType = (typeof IssueType)[keyof typeof IssueType];

// ─────────────────────────────────────────────────────────────────────────────
// Per-flow input data
// ─────────────────────────────────────────────────────────────────────────────

/** Case content submitted by the create-case flow. */
export interface CaseInput {
  /** Goes in the field labelled "Title" — "Case Details" is the section
   * heading above it, not the input. */
  title: string;
  description: string;
  issueType: IssueType;
  severity: Severity;
}

/**
 * Case content per project type. Kept per-type so each project's cases are
 * distinguishable, and because the available severities differ by project.
 *
 * ⚠️ `POST /cases` has no delete counterpart, so every run leaves a permanent
 * record. Keep `description` self-describing so those records stay identifiable
 * in the target environment.
 */
export const CASE_INPUT: Record<ProjectType, CaseInput> = {
  [ProjectType.SUBSCRIPTION]: {
    title: "subscription case",
    description: "This is a test case for subscription project",
    issueType: IssueType.QUESTION,
    severity: Severity.S4,
  },
  [ProjectType.MANAGED_CLOUD_SUBSCRIPTION]: {
    title: "MS subscription case",
    description: "This is a test case for MS subscription project",
    issueType: IssueType.QUESTION,
    severity: Severity.S4,
  },
  [ProjectType.CLOUD_SUPPORT]: {
    title: "Cloud support case",
    description: "This is a test case for cloud support project",
    issueType: IssueType.QUESTION,
    severity: Severity.S4,
  },
};

/** Content submitted by the create-service-request flow.
 *
 * The Request Details fields are **dynamic**: they are rendered from the
 * selected catalog item's ServiceNow variables (`variable.questionText` in
 * VariableFormFields.tsx), so their labels come from the catalog rather than
 * from the frontend. `requestDetailsLabel` and `descriptionLabel` therefore
 * record what those fields are actually called on the target environment. */
export interface ServiceRequestInput {
  /** Catalog accordion to expand, e.g. "Generic Requests". */
  catalog: string;
  /** Radio item to select inside that catalog. Note this is NOT always the same
   * wording as the catalog it lives under. */
  catalogItem: string;
  requestDetails: string;
  description: string;
}

/**
 * Service request content for the Managed Cloud Subscription project.
 *
 * The catalog and item names come from ServiceNow, not the frontend, so they
 * are environment data. Verified live: the "Generic Requests" catalog holds a
 * single item named "General Requests" — the wording genuinely differs.
 *
 * ⚠️ Creates a permanent record on every run, like case creation.
 */
export const SERVICE_REQUEST_INPUT: ServiceRequestInput = {
  catalog: "Generic Requests",
  catalogItem: "General Requests",
  requestDetails: "This is test Generic Request SR for MS sub",
  description: "This is a test Generic Request SR for MS subscription project",
};

/** Content submitted by the create-security-report flow.
 *
 * A security report is a case raised at `/support/security-report/create`. The
 * form hides Issue Type and Severity and requires at least one attachment.
 *
 * There is no `title` here: for security reports CreateCasePage generates it
 * from the selected deployment, the product name and today's date, overwriting
 * anything typed (see the auto-fill effect in CreateCasePage.tsx). */
export interface SecurityReportInput {
  description: string;
  /** Attachment path, relative to the tests/e2e directory. Kept in-repo rather
   * than pointing at a developer's Downloads folder so the spec is portable to
   * other machines and to CI. */
  attachmentPath: string;
}

/**
 * Security report content for the Managed Cloud Subscription project.
 *
 * ⚠️ Creates a permanent record on every run.
 */
export const SECURITY_REPORT_INPUT: SecurityReportInput = {
  description: "This is a test Security Report SR for MS subscription project",
  attachmentPath: "fixtures/files/sraattachment.csv",
};

/**
 * "Request Product Logs" service request, under the Information Request
 * catalog.
 *
 * This catalog item has several variables rather than the single free-text
 * field the Generic Requests item has, including two date/time inputs. The
 * dates are expressed as offsets so each run submits a valid, recent window
 * rather than a hardcoded date that drifts into the past.
 *
 * ⚠️ Creates a permanent record on every run.
 */
export interface ProductLogsRequestInput {
  catalog: string;
  catalogItem: string;
  /** Value for "Types of product log required". */
  logType: string;
  /** Start Time, as whole days before today. */
  startDaysAgo: number;
  /** End Time, as whole days before today. */
  endDaysAgo: number;
  purpose: string;
  description: string;
}

export const PRODUCT_LOGS_REQUEST_INPUT: ProductLogsRequestInput = {
  catalog: "Information Request",
  catalogItem: "Request Product Logs",
  logType: "Carbon",
  startDaysAgo: 3,
  endDaysAgo: 1,
  purpose: "This is a test Information Request SR for MS subscription project",
  description:
    "This is a test Information Request Description SR for MS subscription project",
};

/**
 * Additional severity coverage, Subscription project only.
 *
 * `CASE_INPUT` already covers S4 for every type; these exercise the rest of the
 * range on one project. Which severities a project actually offers comes from
 * its `acceptedSeverityValues`, so this list is deliberately scoped to the
 * Subscription fixture rather than applied to all types.
 *
 * ⚠️ Each entry creates its own permanent case on every run.
 */
export const SUBSCRIPTION_SEVERITY_CASES: CaseInput[] = [
  {
    title: "subscription case S1",
    description: "This is a test case for subscription project with severity S1",
    issueType: IssueType.QUESTION,
    severity: Severity.S1,
  },
  {
    title: "subscription case S2",
    description: "This is a test case for subscription project with severity S2",
    issueType: IssueType.QUESTION,
    severity: Severity.S2,
  },
  {
    title: "subscription case S3",
    description: "This is a test case for subscription project with severity S3",
    issueType: IssueType.QUESTION,
    severity: Severity.S3,
  },
];
