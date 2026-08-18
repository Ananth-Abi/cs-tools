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

import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

// Unlike CsmIssuesView.test.tsx, this file keeps the REAL CasesFilterBar (and
// CasesList) mounted -- the bug under test only reproduces through the real
// round trip of a filter bar interaction -> CsmIssuesView's setFilters ->
// the URL -> re-reading the URL back into `filters`.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn().mockResolvedValue({ teams: [] }), get: vi.fn() }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));
vi.mock("@api/useDirectoryUsers", () => ({
  useDirectoryUsers: () => ({ data: [] }),
}));
vi.mock("@features/csm-cases/api/useGetCsmCases", () => ({
  useGetCsmCases: () => ({
    data: { cases: [], total: 0, hasMore: false },
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
  }),
}));
vi.mock("@components/FilteredCsvExportButton", () => ({
  default: () => <div>ExportButton</div>,
}));
vi.mock("@components/RefreshButton", () => ({
  default: () => <div>RefreshButton</div>,
}));

import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";

function renderAt(initialUrl: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialUrl]}>
        <Routes>
          <Route path="/cases" element={<CsmIssuesView title="Cases" />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmIssuesView + real CasesFilterBar — work state clears fully from the URL", () => {
  // Regression test for: with only "In progress" selected, checking then
  // unchecking a work state left the last-checked value permanently stuck
  // (could only toggle between work states, never reach none). Root cause:
  // CsmIssuesView's FILTER_PARAM_KEYS (the URL keys it deletes before
  // rewriting from the next filter state) didn't include "workStates", so an
  // empty next.workStates never actually cleared the stale URL param, and
  // the next render read the stale value straight back in.
  it("deselecting the only checked work state actually clears it, not just toggles it", () => {
    renderAt("/cases?states=work_in_progress&workStates=ongoing");

    const workState = screen.getByRole("combobox", { name: "Work state" });
    expect(workState).toHaveTextContent("Ongoing");

    // Open the dropdown and uncheck the only selected option.
    fireEvent.mouseDown(workState);
    fireEvent.click(screen.getByRole("option", { name: "Ongoing" }));

    // The control must now show no selection -- not still "Ongoing", and not
    // forced onto "Paused" either.
    expect(workState).not.toHaveTextContent("Ongoing");
    expect(workState).not.toHaveTextContent("Paused");
  });
});
