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

import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router";

const navigateMock = vi.fn();
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));

// Imported after the mock above so the module picks it up.
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";

const DASHED_ID = "56f49f0a-eb1e-c310-fcf5-f5dabad0cdab";
const DASHLESS_ID = "56f49f0aeb1ec310fcf5f5dabad0cdab";

function wrapperAt(initialEntry: string) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/cases/:caseId" element={children} />
        </Routes>
      </MemoryRouter>
    );
  };
}

describe("useNormalizedIdParam", () => {
  beforeEach(() => {
    navigateMock.mockClear();
  });

  it("returns the dashed id and redirects when the route param is a dashless 32-hex id", async () => {
    const { result } = renderHook(() => useNormalizedIdParam("caseId"), {
      wrapper: wrapperAt(`/cases/${DASHLESS_ID}`),
    });

    expect(result.current).toBe(DASHED_ID);

    await waitFor(() =>
      expect(navigateMock).toHaveBeenCalledWith(
        { pathname: `/cases/${DASHED_ID}`, search: "", hash: "" },
        { replace: true },
      ),
    );
  });

  it("preserves the query string and hash on the redirect", async () => {
    const { result } = renderHook(() => useNormalizedIdParam("caseId"), {
      wrapper: wrapperAt(`/cases/${DASHLESS_ID}?tab=comments#section-2`),
    });

    expect(result.current).toBe(DASHED_ID);

    await waitFor(() =>
      expect(navigateMock).toHaveBeenCalledWith(
        {
          pathname: `/cases/${DASHED_ID}`,
          search: "?tab=comments",
          hash: "#section-2",
        },
        { replace: true },
      ),
    );
  });

  it("returns an already-dashed id unchanged and does not navigate", () => {
    const { result } = renderHook(() => useNormalizedIdParam("caseId"), {
      wrapper: wrapperAt(`/cases/${DASHED_ID}`),
    });

    expect(result.current).toBe(DASHED_ID);
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it.each([
    ["too short (31 hex chars)", DASHLESS_ID.slice(0, 31)],
    ["too long (33 hex chars)", `${DASHLESS_ID}a`],
    ["non-hex characters", `${DASHLESS_ID.slice(0, 30)}gz`],
  ])(
    "returns a malformed id unchanged and does not navigate or crash — %s",
    (_desc, malformedId) => {
      const { result } = renderHook(() => useNormalizedIdParam("caseId"), {
        wrapper: wrapperAt(`/cases/${malformedId}`),
      });

      expect(result.current).toBe(malformedId);
      expect(navigateMock).not.toHaveBeenCalled();
    },
  );
});
