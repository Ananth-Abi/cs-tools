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
const useGetCsmCasesMock = vi.fn();
vi.mock("@features/csm-cases/api/useGetCsmCases", () => ({
  useGetCsmCases: (...args: unknown[]) => {
    useGetCsmCasesMock(...args);
    return {
      data: { cases: [], total: 0, hasMore: false },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      dataUpdatedAt: 0,
    };
  },
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
  useGetCsmCasesMock.mockClear();
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
  // Mirrors the real `CsmEngagementsPage`: locked to one case type (so Type
  // is offered but not default-visible — see `isLockedToSingleType`'s doc in
  // `CsmIssuesView`) and severity hidden (Engagements has no severity concept).
  function renderEngagementsLike() {
    return render(
      <MemoryRouter initialEntries={["/engagements"]}>
        <CsmIssuesView
          title="Engagements"
          entityNoun="engagements"
          lockedFilters={{ caseTypes: ["engagement"] }}
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

  it("renders the picker and lists Product/Type/Assignee/Customer/Created (not Severity) when hideSeverityColumn is set", () => {
    renderEngagementsLike();
    fireEvent.click(screen.getByRole("button", { name: "Customise engagements columns" }));

    expect(screen.getByText("Product")).toBeInTheDocument();
    expect(screen.getByText("Type")).toBeInTheDocument();
    expect(screen.getByText("Assignee")).toBeInTheDocument();
    expect(screen.getByText("Customer")).toBeInTheDocument();
    expect(screen.getByText("Created")).toBeInTheDocument();
    expect(screen.queryByText("Severity")).not.toBeInTheDocument();
  });

  it("defaults every optional column but Product to hidden, since the view is locked to one case type, so a returning user sees no change until they opt in", () => {
    renderEngagementsLike();
    fireEvent.click(screen.getByRole("button", { name: "Customise engagements columns" }));

    // Order mirrors `CASE_OPTIONAL_COLUMNS`: Product, Type, Assignee, Customer, Created
    // (Severity is excluded entirely here via hideSeverityColumn).
    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes).toHaveLength(5);
    expect(checkboxes[0]).toBeChecked(); // Product — the one default-visible column
    expect(checkboxes[1]).not.toBeChecked(); // Type
    expect(checkboxes[2]).not.toBeChecked(); // Assignee
    expect(checkboxes[3]).not.toBeChecked(); // Customer
    expect(checkboxes[4]).not.toBeChecked(); // Created
  });

  it("persists a toggled column across a remount for the same user + view", () => {
    const { unmount } = renderEngagementsLike();
    fireEvent.click(screen.getByRole("button", { name: "Customise engagements columns" }));
    fireEvent.click(screen.getByText("Assignee"));
    unmount();

    renderEngagementsLike();
    fireEvent.click(screen.getByRole("button", { name: "Customise engagements columns" }));
    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes[2]).toBeChecked(); // Assignee, toggled on above
  });
});

describe("CsmIssuesView defaultCaseTypes (Support page's case-type default, digiops-cs#2907 follow-up)", () => {
  function renderWithDefault(initialEntry: string) {
    return render(
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route
            path="/cases"
            element={<CsmIssuesView title="Cases" defaultCaseTypes={["case"]} />}
          />
        </Routes>
      </MemoryRouter>,
    );
  }

  it("seeds caseTypes to the default on a fresh visit with no `types` param at all", () => {
    renderWithDefault("/cases");
    expect(useGetCsmCasesMock).toHaveBeenCalledWith(
      expect.objectContaining({ caseTypes: ["case"] }),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
    );
  });

  it("does not override an explicit `types` param already in the URL", () => {
    renderWithDefault("/cases?types=service_request");
    expect(useGetCsmCasesMock).toHaveBeenCalledWith(
      expect.objectContaining({ caseTypes: ["service_request"] }),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
    );
  });

  it("does nothing when hideTypeFilter is set (a locked-type page has no use for a default)", () => {
    render(
      <MemoryRouter initialEntries={["/cases"]}>
        <Routes>
          <Route
            path="/cases"
            element={
              <CsmIssuesView
                title="Service Requests"
                lockedFilters={{ caseTypes: ["service_request"] }}
                hideTypeFilter
                defaultCaseTypes={["case"]}
              />
            }
          />
        </Routes>
      </MemoryRouter>,
    );
    expect(useGetCsmCasesMock).toHaveBeenCalledWith(
      expect.objectContaining({ caseTypes: ["service_request"] }),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
      expect.anything(),
    );
  });
});
