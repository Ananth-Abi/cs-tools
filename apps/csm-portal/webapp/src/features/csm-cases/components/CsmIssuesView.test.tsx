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
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn() }),
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
vi.mock("@features/csm-cases/components/CasesFilterBar", () => ({
  default: () => <div>FilterBar</div>,
}));
vi.mock("@features/csm-cases/components/CasesList", () => ({
  default: () => <div>CasesList</div>,
}));
vi.mock("@components/FilteredCsvExportButton", () => ({
  default: () => <div>ExportButton</div>,
}));
vi.mock("@components/RefreshButton", () => ({
  default: () => <div>RefreshButton</div>,
}));

import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";

beforeEach(() => {
  window.localStorage.clear();
});

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

function renderAt(initialState: unknown, hideBackButton?: boolean) {
  return render(
    <MemoryRouter initialEntries={[{ pathname: "/cases", state: initialState }]}>
      <Routes>
        <Route
          path="/cases"
          element={<CsmIssuesView title="Cases" hideBackButton={hideBackButton} />}
        />
        <Route path="/dashboard" element={<LocationProbe />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("CsmIssuesView back navigation", () => {
  it("renders no Back button when it wasn't reached from a dashboard widget", () => {
    renderAt(null);
    expect(screen.queryByRole("button", { name: "Back" })).not.toBeInTheDocument();
  });

  it("renders a Back button that returns to the dashboard when reached via a dashboard widget's `from` state", () => {
    renderAt({ from: "/dashboard" });

    const backButton = screen.getByRole("button", { name: "Back" });
    fireEvent.click(backButton);

    expect(screen.getByTestId("location-probe")).toHaveTextContent("/dashboard");
  });

  // Regression test: embedding this view as a project's Work items sub-tab
  // still sees the outer page's `from` state (location.state belongs to the
  // route, not this component) and used to render a second, redundant Back
  // button on top of the page-level one. `hideBackButton` must suppress it.
  it("suppresses its own Back button when hideBackButton is set, even with a `from` state present", () => {
    renderAt({ from: "/dashboard" }, true);
    expect(screen.queryByRole("button", { name: "Back" })).not.toBeInTheDocument();
  });
});

describe("CsmIssuesView column customization", () => {
  function renderEngagementsLike() {
    return render(
      <MemoryRouter initialEntries={["/engagements"]}>
        <CsmIssuesView
          title="Engagements"
          entityNoun="engagements"
          hideSeverityColumn
          enableColumnCustomization
          columnsViewId="engagements"
        />
      </MemoryRouter>,
    );
  }

  it("is off by default (no picker rendered for a plain CsmIssuesView)", () => {
    renderAt(null);
    expect(
      screen.queryByRole("button", { name: /Customise .* columns/ }),
    ).not.toBeInTheDocument();
  });

  it("renders the picker and lists Product/Assignee (not Type/Severity) when hideSeverityColumn is set", () => {
    renderEngagementsLike();
    fireEvent.click(screen.getByRole("button", { name: "Customise engagements columns" }));

    expect(screen.getByText("Product")).toBeInTheDocument();
    expect(screen.getByText("Assignee")).toBeInTheDocument();
    expect(screen.queryByText("Type")).not.toBeInTheDocument();
    expect(screen.queryByText("Severity")).not.toBeInTheDocument();
  });

  it("defaults Assignee to hidden so a returning user sees no change until they opt in", () => {
    renderEngagementsLike();
    fireEvent.click(screen.getByRole("button", { name: "Customise engagements columns" }));

    const checkboxes = screen.getAllByRole("checkbox");
    // Product first (checked, default-visible), Assignee second (unchecked).
    expect(checkboxes[0]).toBeChecked();
    expect(checkboxes[1]).not.toBeChecked();
  });

  it("persists a toggled column across a remount for the same user + view", () => {
    const { unmount } = renderEngagementsLike();
    fireEvent.click(screen.getByRole("button", { name: "Customise engagements columns" }));
    fireEvent.click(screen.getByText("Assignee"));
    unmount();

    renderEngagementsLike();
    fireEvent.click(screen.getByRole("button", { name: "Customise engagements columns" }));
    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes[1]).toBeChecked();
  });
});
