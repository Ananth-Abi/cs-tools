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
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

let mockRoles: string[] | undefined = ["admin"];

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { roles: mockRoles }, isLoading: false, isError: false }),
}));
// `CsmAdminLayout` transitively imports the dashboard builder's own admin
// routes (via the nav tree/`useRouteTabs`), some of which reach real API
// hooks — mocked up front, before the component import below, per this
// repo's own convention for anything that transitively imports
// `CsmAdminLayout` (see e.g. `adminRoutes.redirects.test.tsx`).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: vi.fn(), post: vi.fn() }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

import CsmAdminLayout from "@features/csm-admin/pages/CsmAdminLayout";

function renderLayout(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route path="/admin" element={<CsmAdminLayout />}>
          <Route
            path="user-management"
            element={
              <>
                <div>User management tiles</div>
                <LocationProbe />
              </>
            }
          />
          <Route path="user-management/users" element={<div>Users content</div>} />
          <Route path="user-management/roles" element={<div>Roles content</div>} />
          <Route path="user-management/groups" element={<div>Groups content</div>} />
          <Route path="user-management/teams" element={<div>Teams content</div>} />
          <Route path="user-management/permissions" element={<div>Permissions content</div>} />
          <Route path="dashboards" element={<div>Dashboards content</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

describe("CsmAdminLayout — top-level tabs", () => {
  it("shows only User management and Dashboards at the top level, for an admin user", () => {
    mockRoles = ["admin"];
    renderLayout("/admin/dashboards");
    expect(screen.getByRole("tab", { name: "User management" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Dashboards" })).toBeInTheDocument();
    // The five directory pages are only reachable as tiles on the User
    // management landing page, never as top-level tabs.
    expect(screen.queryByRole("tab", { name: "Users" })).not.toBeInTheDocument();
  });

  it("hides the Dashboards tab for a non-admin user, while User management still shows", () => {
    mockRoles = ["agent"];
    renderLayout("/admin/user-management/users");
    expect(screen.queryByRole("tab", { name: "Dashboards" })).not.toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "User management" })).toBeInTheDocument();
  });
});

describe("CsmAdminLayout — back link to the User management tile grid", () => {
  it("shows no back link on the landing route itself", () => {
    mockRoles = ["admin"];
    renderLayout("/admin/user-management");
    expect(screen.getByText("User management tiles")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /back to user management/i }),
    ).not.toBeInTheDocument();
  });

  it("shows a back link on each directory page reached via a tile, and it returns to the landing route", () => {
    mockRoles = ["admin"];
    renderLayout("/admin/user-management/roles");
    expect(screen.getByText("Roles content")).toBeInTheDocument();
    const back = screen.getByRole("button", { name: /back to user management/i });

    fireEvent.click(back);

    expect(screen.getByTestId("location-probe")).toHaveTextContent("/admin/user-management");
    expect(screen.getByText("User management tiles")).toBeInTheDocument();
    expect(screen.queryByText("Roles content")).not.toBeInTheDocument();
  });

  it("does not show the back link when Dashboards is the active top-level tab", () => {
    mockRoles = ["admin"];
    renderLayout("/admin/dashboards");
    expect(screen.getByText("Dashboards content")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /back to user management/i }),
    ).not.toBeInTheDocument();
  });

  // Regression test: the back link used to render below the tab strip,
  // inconsistent with every other page's Back button (always the first
  // thing on the page, above the title). It must precede both the "Settings"
  // heading and the tab strip in document order.
  it("renders the back link above the Settings title and tab strip, not below them", () => {
    mockRoles = ["admin"];
    renderLayout("/admin/user-management/roles");
    const back = screen.getByRole("button", { name: /back to user management/i });
    const heading = screen.getByRole("heading", { name: "Settings" });
    const tabStrip = screen.getByRole("tab", { name: "User management" });

    expect(back.compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(back.compareDocumentPosition(tabStrip) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});
