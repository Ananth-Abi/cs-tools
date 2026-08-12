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
// Waiting on a list's search results.
//
// Every searchable list in the portal — the cases list and the Security Report
// Analysis list included — is backed by the same `POST /projects/{id}/cases/search`
// endpoint, so one helper serves them all.
//

import { type Page, type Response } from "../fixtures/test";

/** How long to allow for a debounced search to fire and return. */
const SEARCH_TIMEOUT_MS = 60_000;

/**
 * Reads the search term out of a `/cases/search` request body.
 *
 * @param postData - Raw request body, or null when there is none.
 * @returns The search term, or undefined if absent or unparseable.
 */
function searchQueryOf(postData: string | null): string | undefined {
  if (!postData) return undefined;
  try {
    // The term travels inside `filters`, alongside caseTypes — verified against a
    // live request. It is NOT at the root of the body.
    const body = JSON.parse(postData) as {
      filters?: { searchQuery?: string };
    };
    return body.filters?.searchQuery;
  } catch {
    return undefined;
  }
}

/**
 * Starts waiting for the search response produced by a specific term.
 *
 * Call this **before** typing, then await the returned promise afterwards.
 *
 * Matching the request's own term is what makes this reliable. Waiting for
 * `networkidle` instead is too weak: it can resolve before the debounced request
 * has even been issued, leaving a caller to read stale rows and conclude a record
 * does not exist — which, for the specs that create on absence, means a permanent
 * duplicate that cannot be deleted.
 *
 * @param page - Test page.
 * @param searchText - The term about to be typed into the list's search box.
 * @returns Promise for the matching search response.
 */
export function caseSearchResponse(
  page: Page,
  searchText: string,
): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      const request = response.request();
      if (request.method() !== "POST") return false;
      if (!new URL(response.url()).pathname.endsWith("/cases/search")) {
        return false;
      }
      return searchQueryOf(request.postData()) === searchText;
    },
    { timeout: SEARCH_TIMEOUT_MS },
  );
}
