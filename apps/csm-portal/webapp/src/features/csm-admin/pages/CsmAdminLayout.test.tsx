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

function renderLayout() {
  return render(
    <MemoryRouter initialEntries={["/admin/users"]}>
      <Routes>
        <Route path="/admin" element={<CsmAdminLayout />}>
          <Route path="users" element={<div>Users content</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

describe("CsmAdminLayout — Dashboards tab gating", () => {
  it("shows the Dashboards tab for an admin user", () => {
    mockRoles = ["admin"];
    renderLayout();
    expect(screen.getByRole("tab", { name: "Dashboards" })).toBeInTheDocument();
  });

  it("hides the Dashboards tab for a non-admin user, while every other tab still shows", () => {
    mockRoles = ["agent"];
    renderLayout();
    expect(screen.queryByRole("tab", { name: "Dashboards" })).not.toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Users" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Roles" })).toBeInTheDocument();
  });
});
