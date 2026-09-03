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

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter, Route, Routes } from "react-router";

let mockUser: { roles?: string[] } | undefined = { roles: ["admin"] };
let mockIsLoading = false;
let mockIsError = false;

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: mockUser, isLoading: mockIsLoading, isError: mockIsError }),
}));

import DashboardBuilderRouteGuard from "@features/csm-admin/dashboards/pages/DashboardBuilderRouteGuard";

function renderGuard() {
  return render(
    <MemoryRouter initialEntries={["/admin/dashboards"]}>
      <Routes>
        <Route path="/admin/dashboards" element={<DashboardBuilderRouteGuard />}>
          <Route index element={<div>Dashboard builder content</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

describe("DashboardBuilderRouteGuard", () => {
  it("renders the guarded route for an admin user", () => {
    mockUser = { roles: ["admin"] };
    mockIsLoading = false;
    mockIsError = false;
    renderGuard();
    expect(screen.getByText("Dashboard builder content")).toBeInTheDocument();
  });

  it("renders a 403 instead of the route for a non-admin user", () => {
    mockUser = { roles: ["agent"] };
    mockIsLoading = false;
    mockIsError = false;
    renderGuard();
    expect(screen.queryByText("Dashboard builder content")).not.toBeInTheDocument();
    expect(screen.getByText(/admin role/i)).toBeInTheDocument();
  });

  it("shows a loading state rather than a premature 403 while the profile is still loading", () => {
    mockUser = undefined;
    mockIsLoading = true;
    mockIsError = false;
    const { container } = renderGuard();
    expect(screen.queryByText("Dashboard builder content")).not.toBeInTheDocument();
    expect(screen.queryByText(/admin role/i)).not.toBeInTheDocument();
    expect(container.querySelector(".MuiSkeleton-root")).toBeInTheDocument();
  });

  it("denies rather than hangs forever when the profile fetch errored", () => {
    mockUser = undefined;
    mockIsLoading = true;
    mockIsError = true;
    renderGuard();
    expect(screen.getByText(/admin role/i)).toBeInTheDocument();
  });
});
