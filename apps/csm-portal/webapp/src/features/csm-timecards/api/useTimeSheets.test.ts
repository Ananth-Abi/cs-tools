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

import { describe, expect, it, vi } from "vitest";
import { mapTimeCard, searchTimeCards } from "@features/csm-timecards/api/useTimeSheets";
import type { BackendApi } from "@api/backend/client";
import type {
  BeSearchTimeCardsPayload,
  BeSearchTimeCardsResponse,
  BeTimeCardState,
  BeTimeCardView,
} from "@api/backend/types";

// useTimeSheets.ts pulls in @config/apiConfig transitively (via useGetUsersMe)
// and @api/backend/client directly; both read window.config at module load
// and throw outside a configured runtime. Mock both so importing
// searchTimeCards below doesn't trip either (vi.mock calls are hoisted above
// these imports by vitest). searchTimeCards takes its own BackendApi instance
// directly, so neither the hook nor the real config is ever exercised here.
vi.mock("@api/backend/client", () => ({ useBackendApi: vi.fn() }));
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));

function beCard(id: string, state: BeTimeCardState): BeTimeCardView {
  return {
    id,
    totalTime: 30,
    workDate: "2026-07-13",
    createdOn: "2026-07-13",
    hasBillable: true,
    state,
    user: { id: "user-1", name: "Jane Doe" },
    project: { id: "proj-1", name: "Acme" },
    case: { id: "case-1", number: "CS0000001", name: "Issue" },
  };
}

function bePage(timeCards: BeTimeCardView[], total: number, offset: number, limit: number): BeSearchTimeCardsResponse {
  return { timeCards, total, offset, limit };
}

/** A mock `BackendApi` whose `post` resolves to `page` and records every
 * payload it was called with, so a test can assert on exactly what
 * `searchTimeCards` forwarded on the wire. */
function mockApi(page: BeSearchTimeCardsResponse): { api: BackendApi; post: ReturnType<typeof vi.fn> } {
  const post = vi.fn(async () => page);
  const api: BackendApi = {
    get: vi.fn(),
    post: post as unknown as BackendApi["post"],
    patch: vi.fn(),
    postEmpty: vi.fn(),
    del: vi.fn(),
    getBlob: vi.fn(),
  };
  return { api, post };
}

function lastPayload(post: ReturnType<typeof vi.fn>): BeSearchTimeCardsPayload {
  return post.mock.calls.at(-1)![1] as BeSearchTimeCardsPayload;
}

describe("mapTimeCard", () => {
  // Confirmed live against the real entity-service (POST /time-cards/search):
  // the per-activity minute breakdown and issue complexity ARE echoed back on
  // read, both for a just-created card and a pre-existing one — a previously
  // documented "write-only" claim that turned out to be wrong.
  it("maps the per-activity minute breakdown and issue complexity through from the wire, when present", () => {
    const withBreakdown: BeTimeCardView = {
      ...beCard("a", "submitted"),
      timeAnalyzing: 11,
      timeSettingUp: 22,
      timeReproducingDebugging: 33,
      timeProvidingSolution: 44,
      timePatching: 55,
      issueComplexity: "High",
    };

    const result = mapTimeCard(withBreakdown);

    expect(result.breakdown).toEqual({
      analysisDebugging: 11,
      settingUp: 22,
      reproduce: 33,
      providingSolution: 44,
      answering: 55,
    });
    expect(result.issueComplexity).toBe("High");
  });

  it("leaves breakdown/issueComplexity undefined when the wire response has none (a card logged before these fields were mapped)", () => {
    const result = mapTimeCard(beCard("a", "submitted"));

    expect(result.breakdown).toBeUndefined();
    expect(result.issueComplexity).toBeUndefined();
  });

  it("treats an unrecognized issueComplexity value as undefined rather than passing it through uncast", () => {
    const result = mapTimeCard({
      ...beCard("a", "submitted"),
      issueComplexity: "Some Future Value",
    });

    expect(result.issueComplexity).toBeUndefined();
  });
});

describe("searchTimeCards", () => {
  it("makes a single request and returns the response as-is", async () => {
    const cards = [beCard("a", "submitted"), beCard("b", "approved")];
    const { api, post } = mockApi(bePage(cards, 2, 0, 20));

    const result = await searchTimeCards(api, undefined, { page: 0, rowsPerPage: 20 });

    expect(post).toHaveBeenCalledTimes(1);
    expect(result.cards.map((c) => c.id)).toEqual(["a", "b"]);
    expect(result.total).toBe(2);
  });

  it("maps workLogComment through from the wire response, when present", async () => {
    const withComment: BeTimeCardView = {
      ...beCard("a", "submitted"),
      workLogComment: "<p>Investigated the reported latency issue.</p>",
    };
    const { api } = mockApi(bePage([withComment, beCard("b", "submitted")], 2, 0, 20));

    const result = await searchTimeCards(api, undefined, { page: 0, rowsPerPage: 20 });

    expect(result.cards[0].workLogComment).toBe(
      "<p>Investigated the reported latency issue.</p>",
    );
    expect(result.cards[1].workLogComment).toBeUndefined();
  });

  // states used to be filtered client-side over one raw page (with a
  // walk-forward workaround for the 500 that combining it with projectIds
  // used to cause); both are fixed upstream now, so this just forwards.
  it("forwards states directly on the wire, in a single request", async () => {
    const { api, post } = mockApi(bePage([beCard("a", "approved")], 1, 0, 20));

    await searchTimeCards(api, { states: ["approved", "rejected"] }, { page: 0, rowsPerPage: 20 });

    expect(post).toHaveBeenCalledTimes(1);
    expect(lastPayload(post).filters?.states).toEqual(["approved", "rejected"]);
  });

  // caseId used to always return total: 0 (worked around via projectIds +
  // client-side filtering in useCaseTimeCards); fixed upstream now.
  it("forwards caseId directly on the wire", async () => {
    const { api, post } = mockApi(bePage([], 0, 0, 20));

    await searchTimeCards(api, { caseId: "case-123" }, { page: 0, rowsPerPage: 20 });

    expect(lastPayload(post).filters?.caseId).toBe("case-123");
  });

  it("forwards userIds directly on the wire, alongside the single-user userId", async () => {
    const { api, post } = mockApi(bePage([], 0, 0, 20));

    await searchTimeCards(
      api,
      { userId: "me", userIds: ["eng-1", "eng-2"] },
      { page: 0, rowsPerPage: 20 },
    );

    const filters = lastPayload(post).filters;
    expect(filters?.userId).toBe("me");
    expect(filters?.userIds).toEqual(["eng-1", "eng-2"]);
  });

  it("omits caseId/states/userIds from the payload when unset", async () => {
    const { api, post } = mockApi(bePage([], 0, 0, 20));

    await searchTimeCards(api, undefined, { page: 0, rowsPerPage: 20 });

    const filters = lastPayload(post).filters;
    expect(filters?.caseId).toBeUndefined();
    expect(filters?.states).toBeUndefined();
    expect(filters?.userIds).toBeUndefined();
  });
});
