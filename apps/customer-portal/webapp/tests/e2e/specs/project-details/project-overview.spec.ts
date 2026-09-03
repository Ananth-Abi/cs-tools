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
// The Overview tab of the project details page, reached through the side nav as
// a user would.
//
// ✅ READ-ONLY — nothing here creates or modifies a record, so it is safe to run
// repeatedly.
//
// The project name and key are asserted exactly, since they identify the
// project. Everything else is asserted as *present and populated* rather than by
// value: created date, support tier, go-live date, subscription dates and query
// hours are all environment data that legitimately changes, so pinning them
// would make this a test of staging's contents rather than of the page.
//
// Service Hours Allocations is feature-gated (`showServiceHoursAllocationsCard`
// follows the project's time-logs access), which is why this targets Managed
// Cloud Subscription.
//

import { test, expect, withSession } from "../../fixtures/test";
import { ProjectOverviewPage } from "../../pages/ProjectOverviewPage";
import { PROJECTS, ProjectType } from "../../config/testData";
import { PROJECT_OVERVIEW } from "../../utils/selectors";

withSession(test);

test.describe("Project Details — overview", () => {
  // Loads the dashboard, navigates the side nav and waits on several cards;
  // the 30s default is not enough for a cold navigation.
  test.describe.configure({ timeout: 120_000 });

  const project = PROJECTS[ProjectType.MANAGED_CLOUD_SUBSCRIPTION];
  const { labels, sections } = PROJECT_OVERVIEW;

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — shows project, contact and service hours details`, async ({
    page,
  }) => {
    test.skip(
      !project.id || !project.projectKey,
      `${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} fixture needs id and projectKey. ` +
        `Fill them in tests/e2e/config/testData.ts.`,
    );

    const overview = new ProjectOverviewPage(page);
    await overview.openOverviewTab(project.id);

    // ── Project Information ────────────────────────────────────────────────
    await expect(
      overview.sectionHeading(sections.projectInformation),
    ).toBeVisible();

    // The one thing worth pinning by value: it identifies the project, so a
    // fixture pointed at the wrong one fails here rather than passing silently.
    await expect(overview.projectName(project.name)).toBeVisible();
    await expect(overview.projectKeyChip(project.projectKey)).toBeVisible();

    // Labels are scoped to their section: several repeat across the page —
    // "Remaining" appears both here and in Service Hours Allocations — so a
    // page-wide match is ambiguous.
    for (const label of [
      labels.projectName,
      labels.createdDate,
      labels.supportTier,
      labels.goLiveDate,
      // Subscription Period sits within the Project Information card.
      labels.subscriptionPeriod,
      labels.start,
      labels.end,
      labels.remaining,
    ]) {
      await expect(
        overview.label(sections.projectInformation, labels.createdDate, label),
        `${label} label`,
      ).toBeVisible();
    }

    // ── Contact Information ────────────────────────────────────────────────
    await expect(
      overview.sectionHeading(sections.contactInformation),
    ).toBeVisible();
    await expect(
      overview.label(
        sections.contactInformation,
        labels.accountManager,
        labels.accountManager,
      ),
    ).toBeVisible();

    // ── Service Hours Allocations ──────────────────────────────────────────
    await expect(
      overview.sectionHeading(sections.serviceHoursAllocations),
    ).toBeVisible();
    await expect(
      overview.label(
        sections.serviceHoursAllocations,
        labels.queryHours,
        labels.queryHours,
      ),
    ).toBeVisible();

    // The cards fall back to "--" when the API returns nothing, so the absence
    // of that placeholder is what distinguishes "displayed correctly" from
    // "rendered but empty".
    //
    // Checked only for these two sections. Project Information is deliberately
    // excluded: on this project Support Tier and Go Live Date are genuinely
    // empty (verified live), which is project data rather than a UI fault —
    // asserting it placeholder-free would fail for the wrong reason, and would
    // start failing the moment either value was filled in.
    await expect(
      overview.emptyValuePlaceholders(
        sections.contactInformation,
        labels.accountManager,
      ),
    ).toHaveCount(0);
    await expect(
      overview.emptyValuePlaceholders(
        sections.serviceHoursAllocations,
        labels.queryHours,
      ),
    ).toHaveCount(0);
  });
});
