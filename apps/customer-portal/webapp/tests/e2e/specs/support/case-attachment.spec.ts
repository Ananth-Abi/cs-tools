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
// The attachment lifecycle on a case: upload, list, expand, collapse, download
// and delete — run against every project type.
//
// Two cases per project, and the split matters:
//
// - The **kept** case holds one upload permanently. The list, expand, collapse
//   and download tests all need a file to be there, and the expand test needs
//   specifically an image. Its upload is guarded on the file name, so a run only
//   uploads when the file is missing — otherwise every run would stack another
//   identical copy with nothing removing them.
// - The **delete** case is used for the upload-then-remove round trip, so it ends
//   exactly where it started. Kept separate so the delete never takes away the
//   fixture the other tests rely on.
//
// Attachments are the only records in this suite that can be deleted, which is
// what makes that round trip possible at all.
//

import fs from "node:fs";
import path from "node:path";
import { test, expect, withSession, type Page } from "../../fixtures/test";
import { CaseAttachmentsPage } from "../../pages/CaseAttachmentsPage";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import {
  ATTACHMENT_DATE_PATTERN,
  ATTACHMENT_FILES,
  ATTACHMENT_TARGETS,
  PROJECTS,
} from "../../config/testData";
import { CASE_ATTACHMENTS } from "../../utils/selectors";
import { expectSuccess } from "../../utils/caseFlows";

withSession(test);

/** Today's date as the attachment list renders it, e.g. "Aug 13, 2026". */
function todayLabel(): string {
  return new Date().toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

test.describe("Case Attachment", () => {
  // Each test is a cold case load plus a tab switch, and the image preview holds
  // a 4s skeleton before rendering; the 30s default is not enough.
  test.describe.configure({ timeout: 180_000 });

  const kept = ATTACHMENT_FILES.kept;
  const transient = ATTACHMENT_FILES.transient;

  for (const target of ATTACHMENT_TARGETS) {
    const project = PROJECTS[target.projectType];

    test.describe(target.projectType, () => {
      /** Opens the kept case's Attachments tab. */
      async function openKeptCase(page: Page): Promise<CaseAttachmentsPage> {
        const caseDetail = new CaseDetailPage(page);
        await caseDetail.open(project.id, target.caseId);
        const attachments = new CaseAttachmentsPage(page);
        await attachments.openTab();
        return attachments;
      }

      test("uploads an attachment to the case", async ({ page }) => {
        test.skip(
          !project.id || !target.caseId,
          `${target.projectType} needs a project id and a case id. ` +
            `Fill them in tests/e2e/config/testData.ts.`,
        );

        const attachments = await openKeptCase(page);

        // Upload only when absent: the file is left in place for the other tests,
        // so an unguarded upload would add a copy on every run.
        const alreadyAttached =
          (await attachments.attachment(kept.name).count()) > 0;

        if (alreadyAttached) {
          console.log(`${target.projectType}: ${kept.name} already attached`);
        } else {
          console.log(`${target.projectType}: ${kept.name} missing, uploading`);
          const response = await attachments.upload(kept.path);
          await expectSuccess(response, "upload");
        }

        // Listed under its own name. Asserted as "at least one row" rather than by
        // visibility: the list renders each row twice for responsive layout, and a
        // multi-match locator would be a strict-mode violation.
        await expect(attachments.attachment(kept.name)).not.toHaveCount(0);

        const row = attachments.attachmentRow(kept.name);
        await expect(row).toContainText(kept.name);
        await expect(row).toContainText(kept.size);

        // Today's date only holds on the run that actually uploaded; later runs
        // keep the original date, so those assert a date is rendered at all.
        await expect(row).toContainText(
          alreadyAttached ? ATTACHMENT_DATE_PATTERN : todayLabel(),
        );
      });

      test("lists the case attachments", async ({ page }) => {
        test.skip(
          !project.id || !target.caseId,
          `${target.projectType} needs a project id and a case id.`,
        );

        const attachments = await openKeptCase(page);

        await expect(attachments.emptyMessage()).toHaveCount(0);
        await expect(attachments.rows()).not.toHaveCount(0);

        // The tab's own count must agree with the number of rows — a mismatch
        // means one of the two is stale.
        const names = await attachments.listedFileNames();
        expect(await attachments.tabCount()).toBe(names.length);

        for (const name of names) {
          const row = attachments.attachmentRow(name);
          await expect
            .soft(row, `${name}: size`)
            .toContainText(CASE_ATTACHMENTS.row.sizePattern);
          await expect
            .soft(row, `${name}: uploader`)
            .toContainText(CASE_ATTACHMENTS.row.uploadedByPrefix);
          await expect
            .soft(row, `${name}: upload date`)
            .toContainText(CASE_ATTACHMENTS.row.datePattern);
        }
      });

      test("expands an image attachment and loads the preview", async ({
        page,
      }) => {
        test.skip(
          !project.id || !target.caseId,
          `${target.projectType} needs a project id and a case id.`,
        );

        const attachments = await openKeptCase(page);

        // The preview toggle only renders for image attachments, so this depends
        // on the kept upload being the PNG.
        await expect(attachments.attachment(kept.name)).not.toHaveCount(0);
        await attachments.expandImage(kept.name);

        // The toggle flips once expanded — the row's own confirmation that the
        // preview is open rather than merely present.
        await expect(attachments.collapseImageButton()).toBeVisible();
      });

      test("collapses an expanded image attachment", async ({ page }) => {
        test.skip(
          !project.id || !target.caseId,
          `${target.projectType} needs a project id and a case id.`,
        );

        const attachments = await openKeptCase(page);

        // Expand first: collapsing is only meaningful from an open preview, and
        // starting from a known state keeps this independent of the sibling test.
        await attachments.expandImage(kept.name);
        await attachments.collapseImage(kept.name);
      });

      test("downloads an attachment", async ({ page }) => {
        test.skip(
          !project.id || !target.caseId,
          `${target.projectType} needs a project id and a case id.`,
        );

        const attachments = await openKeptCase(page);
        const download = await attachments.download(kept.name);

        // What a user would end up with on disk, from the anchor's `download`
        // attribute.
        expect(download.suggestedFilename()).toBe(kept.name);

        // Comparing bytes against the fixture proves the content actually came
        // back, not merely that a download was triggered. Read from the fixture
        // rather than a hardcoded number, so replacing the file keeps both sides
        // in step.
        const downloadedPath = await download.path();
        expect(downloadedPath, "download produced no file").toBeTruthy();

        const fixturePath = path.join(process.cwd(), "tests", "e2e", kept.path);
        expect(fs.statSync(downloadedPath as string).size).toBe(
          fs.statSync(fixturePath).size,
        );
      });

      test("deletes an attachment", async ({ page }) => {
        test.skip(
          !project.id || !target.deleteCaseId,
          `${target.projectType} needs a project id and a delete case id.`,
        );

        const caseDetail = new CaseDetailPage(page);
        await caseDetail.open(project.id, target.deleteCaseId);

        const attachments = new CaseAttachmentsPage(page);
        await attachments.openTab();

        // Clear anything a previously failed run left behind, so the upload below
        // starts from a known-empty state rather than adding a second copy.
        await attachments.deleteAll(transient.name);

        const response = await attachments.upload(transient.path);
        await expectSuccess(response, "upload");

        const row = attachments.attachmentRow(transient.name);
        await expect(row).toContainText(transient.name);
        await expect(row).toContainText(transient.size);
        await expect(row).toContainText(todayLabel());

        // The row's control opens a "Confirm Action" dialog, which deleteAll
        // asserts on before confirming.
        await attachments.deleteAll(transient.name);

        // Gone from the list, so the case ends as it started.
        await expect(attachments.attachment(transient.name)).toHaveCount(0);
        await expect(attachments.deleteButton(transient.name)).toHaveCount(0);
      });
    });
  }
});
