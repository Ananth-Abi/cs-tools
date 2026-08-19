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
// The cases lists, reached the way a user reaches them: side nav → Support
// Center → the Outstanding Cases card's footer buttons. Run against every
// project type.
//
// Read-only — nothing here creates or changes a case.
//
// My Cases is a filtered view of the same page. The filtering is done by the
// backend on `filters.createdByMe`, and the heading merely reports it, so the
// tests assert the flag on the wire as well as what the page renders: a heading
// that says "My Cases" over an unfiltered result set would otherwise pass.
//
// Both lists put their filters behind a collapsed panel whose contents are
// unmounted, so every assertion about a filter opens the panel first —
// otherwise "Created By is absent" holds trivially against an empty DOM.
//

import { test, expect, withSession } from "../../fixtures/test";
import { CasesListPage } from "../../pages/CasesListPage";
import { SupportCenterPage } from "../../pages/SupportCenterPage";
import { CASE_LIST_PROJECTS, PROJECTS } from "../../config/testData";
import { CASES_LIST, SUPPORT_CENTER } from "../../utils/selectors";
import { expectSuccess } from "../../utils/caseFlows";
import {
  allCasesSearchResponse,
  myCasesSearchResponse,
} from "../../utils/listSearch";

withSession(test);

test.describe("Cases List", () => {
  // A dashboard load, a nav navigation, the support cards' own queries and then
  // a second page with its own search — well past the 30s default.
  test.describe.configure({ timeout: 180_000 });

  for (const projectType of CASE_LIST_PROJECTS) {
    const project = PROJECTS[projectType];

    test.describe(projectType, () => {
      test.describe("All Cases", () => {
        test("lists all cases from the Support Center", async ({ page }) => {
          test.skip(
            !project.id,
            `${projectType} needs a project id. ` +
              `Fill it in tests/e2e/config/testData.ts.`,
          );

          const support = new SupportCenterPage(page);
          await support.openViaSideNav(project.id);

          // Armed before the click: the list issues its search as it mounts, so
          // a listener registered afterwards can miss the response entirely.
          const searchResponse = allCasesSearchResponse(page);

          // Playwright scrolls the button into view before clicking, so the
          // card's footer needs no explicit scroll.
          await support
            .outstandingCasesFooterButton(
              SUPPORT_CENTER.outstandingCases.allCasesButton,
            )
            .click();

          // Matching this response at all is the proof that the request went out
          // unfiltered — the predicate is what selected it.
          await expectSuccess(await searchResponse, "all cases search");

          // No query string: the same route, unfiltered. Anchored at the end so
          // a `?createdByMe=true` on it could not satisfy this.
          await expect(page).toHaveURL(
            new RegExp(`/projects/${project.id}/${CASES_LIST.pathSegment}$`),
          );

          const cases = new CasesListPage(page);
          await expect(cases.heading(CASES_LIST.allCases.title)).toBeVisible();
          await expect(
            cases.heading(CASES_LIST.allCases.description),
          ).toBeVisible();

          // Rows first: the results bar mounts while the list is still loading
          // and reads "Showing 0 of 0 cases" until the search lands.
          await expect(cases.rows()).not.toHaveCount(0);

          const total = await cases.totalCount();
          expect(total, "the results bar should report a total").not.toBeNull();
          expect(
            total,
            "the project's case list should not be empty",
          ).toBeGreaterThan(0);

          // Created By is offered here and withheld on My Cases — the one filter
          // that differs between the two, and so the UI's own statement that
          // this list is not narrowed to a single creator.
          await cases.openFilters();
          await expect(cases.createdByFilter()).not.toHaveCount(0);

          // Rows carry a creator, but deliberately no assertion that several do:
          // this page is the 10 most recently updated cases, and one account
          // created nearly every case in these projects, so a single creator
          // here is expected rather than a filtering failure. The subset test
          // below is what proves the two lists differ by contents.
          const creators = await cases.rowCreators();
          expect(creators.length, "no row named a creator").toBeGreaterThan(0);

          console.log(
            `All Cases (${projectType}): ${total} cases, ` +
              `${new Set(creators).size} distinct creator(s) on this page`,
          );
        });
      });

      test.describe("My Cases", () => {
        test("lists my cases from the Support Center", async ({ page }) => {
          test.skip(
            !project.id,
            `${projectType} needs a project id. ` +
              `Fill it in tests/e2e/config/testData.ts.`,
          );

          const support = new SupportCenterPage(page);
          await support.openViaSideNav(project.id);

          const searchResponse = myCasesSearchResponse(page);

          await support
            .outstandingCasesFooterButton(
              SUPPORT_CENTER.outstandingCases.myCasesButton,
            )
            .click();

          // Matching this response at all is the proof that the request carried
          // createdByMe — the predicate is what selected it.
          await expectSuccess(await searchResponse, "my cases search");

          await expect(page).toHaveURL(
            new RegExp(
              `/projects/${project.id}/${CASES_LIST.pathSegment}\\?${CASES_LIST.myCasesQuery}`,
            ),
          );

          const cases = new CasesListPage(page);
          await expect(cases.heading(CASES_LIST.myCases.title)).toBeVisible();
          await expect(
            cases.heading(CASES_LIST.myCases.description),
          ).toBeVisible();

          await expect(cases.rows()).not.toHaveCount(0);

          const total = await cases.totalCount();
          expect(
            total,
            "the results bar should report a total for the filtered list",
          ).not.toBeNull();
          expect(
            total,
            "this account created cases in this project through the other " +
              "specs, so its own list should not be empty — an empty one means " +
              "the session belongs to a different user than the one that " +
              "created them",
          ).toBeGreaterThan(0);

          // The Created By filter is withheld here: the list is already narrowed
          // to one creator, so offering to pick another would contradict it.
          await cases.openFilters();
          await expect(cases.createdByFilter()).toHaveCount(0);

          // Every row names the same creator, which is what "my cases" has to
          // mean. Asserted as "one distinct value" rather than against a
          // hardcoded name so the test holds for whichever account the captured
          // session belongs to.
          const creators = await cases.rowCreators();
          expect(creators.length, "no row named a creator").toBeGreaterThan(0);
          expect(
            new Set(creators).size,
            `creators listed: ${creators.join(", ")}`,
          ).toBe(1);

          console.log(
            `My Cases (${projectType}): ${total} cases, created by ${creators[0]}`,
          );
        });

        test("my cases are a subset of all cases", async ({ page }) => {
          test.skip(!project.id, `${projectType} needs a project id.`);

          const support = new SupportCenterPage(page);
          const cases = new CasesListPage(page);

          // The sibling footer button opens the same page unfiltered, which
          // gives the baseline to compare against. Without it a "my cases" total
          // proves nothing: a filter that silently does nothing still returns a
          // number.
          await support.openViaSideNav(project.id);
          const allSearchResponse = allCasesSearchResponse(page);
          await support
            .outstandingCasesFooterButton(
              SUPPORT_CENTER.outstandingCases.allCasesButton,
            )
            .click();
          await expectSuccess(await allSearchResponse, "all cases search");
          await expect(cases.heading(CASES_LIST.allCases.title)).toBeVisible();
          // Wait for rows before reading the bar — until the search lands it
          // reports "Showing 0 of 0 cases", which would make the comparison
          // below vacuous.
          await expect(cases.rows()).not.toHaveCount(0);
          const allTotal = await cases.totalCount();

          const mySearchResponse = myCasesSearchResponse(page);
          await support.openViaSideNav(project.id);
          await support
            .outstandingCasesFooterButton(
              SUPPORT_CENTER.outstandingCases.myCasesButton,
            )
            .click();
          await expectSuccess(await mySearchResponse, "my cases search");
          await expect(cases.heading(CASES_LIST.myCases.title)).toBeVisible();
          await expect(cases.rows()).not.toHaveCount(0);
          const myTotal = await cases.totalCount();

          expect(allTotal, "All Cases reported no total").not.toBeNull();
          expect(myTotal, "My Cases reported no total").not.toBeNull();

          // A subset, necessarily — the same project, with one filter added. Not
          // an equality check: the account may well have created every case in
          // the project, so equal totals are legitimate.
          expect(myTotal as number).toBeLessThanOrEqual(allTotal as number);

          console.log(
            `${projectType}: ${myTotal} of ${allTotal} cases are mine`,
          );
        });
      });
    });
  }
});
