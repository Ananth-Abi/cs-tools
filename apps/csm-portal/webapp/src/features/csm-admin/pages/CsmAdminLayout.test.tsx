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
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router";

let mockRoles: string[] | undefined = ["admin"];

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { roles: mockRoles }, isLoading: false, isError: false }),
}));

import CsmAdminLayout from "@features/csm-admin/pages/CsmAdminLayout";

function renderLayout(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route path="/admin" element={<CsmAdminLayout />}>
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
    // Dashboards active, so the nested User management strip is absent —
    // isolates the top-level strip's own two tabs from its sub-tabs.
    renderLayout("/admin/dashboards");
    expect(screen.getByRole("tab", { name: "User management" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Dashboards" })).toBeInTheDocument();
    // The five directory pages are only reachable as User management's nested
    // sub-tabs, never as top-level tabs.
    expect(screen.queryByRole("tab", { name: "Users" })).not.toBeInTheDocument();
  });

  it("hides the Dashboards tab for a non-admin user, while User management still shows", () => {
    mockRoles = ["agent"];
    renderLayout("/admin/user-management/users");
    expect(screen.queryByRole("tab", { name: "Dashboards" })).not.toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "User management" })).toBeInTheDocument();
  });
});

describe("CsmAdminLayout — nested User management tabs", () => {
  it("shows User management's five sub-tabs when it is the active top-level tab", () => {
    mockRoles = ["admin"];
    renderLayout("/admin/user-management/users");
    expect(screen.getByRole("tab", { name: "User management" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    for (const label of ["Users", "Roles", "Groups", "Teams", "Permissions"]) {
      expect(screen.getByRole("tab", { name: label })).toBeInTheDocument();
    }
  });

  it("marks the sub-tab matching the current route as active", () => {
    mockRoles = ["admin"];
    renderLayout("/admin/user-management/roles");
    expect(screen.getByRole("tab", { name: "Roles" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText("Roles content")).toBeInTheDocument();
  });

  it("deep-links directly to a sub-page with both levels active and the right outlet rendered", () => {
    mockRoles = ["admin"];
    renderLayout("/admin/user-management/groups");
    expect(screen.getByRole("tab", { name: "User management" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByRole("tab", { name: "Groups" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText("Groups content")).toBeInTheDocument();
  });

  it("does not render the nested strip when Dashboards is the active top-level tab", () => {
    mockRoles = ["admin"];
    renderLayout("/admin/dashboards");
    expect(screen.getByRole("tab", { name: "Dashboards" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.queryByRole("tab", { name: "Users" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Roles" })).not.toBeInTheDocument();
    expect(screen.getByText("Dashboards content")).toBeInTheDocument();
  });
});
