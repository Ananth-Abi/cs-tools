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

import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";
import type { ReactNode } from "react";

// Records each `activeItem` CsmSideBar hands the real Sidebar, without
// reassigning an outer-scope variable during render (a mock function call
// is not a render side effect the way a bare assignment is).
const sidebarPropsSpy = vi.fn();

function lastActiveItem(): string | undefined {
  return sidebarPropsSpy.mock.calls.at(-1)?.[0] as string | undefined;
}

// The real Sidebar's internal DOM/selection markup isn't this test's concern
// -- only what `activeItem` CsmSideBar computed and handed it. Every
// compound sub-component it renders (`Sidebar.Nav`, `.Item`, ...) is stubbed
// to a passthrough so CsmSideBar's own JSX still resolves.
vi.mock("@wso2/oxygen-ui", async () => {
  const actual = await vi.importActual<typeof import("@wso2/oxygen-ui")>("@wso2/oxygen-ui");
  const Passthrough = ({ children }: { children?: ReactNode }) => <>{children}</>;
  function MockSidebar({
    activeItem,
    children,
  }: {
    activeItem?: string;
    children?: ReactNode;
  }) {
    sidebarPropsSpy(activeItem);
    return <div data-testid="sidebar">{children}</div>;
  }
  MockSidebar.Nav = Passthrough;
  MockSidebar.Category = Passthrough;
  MockSidebar.CategoryLabel = Passthrough;
  MockSidebar.Item = Passthrough;
  MockSidebar.ItemIcon = Passthrough;
  MockSidebar.ItemLabel = Passthrough;
  MockSidebar.ItemBadge = Passthrough;
  MockSidebar.Footer = Passthrough;
  return { ...actual, Sidebar: MockSidebar };
});

import CsmSideBar from "@components/side-nav-bar/CsmSideBar";

const LAST_SECTION_KEY = "csm.sidebar.lastSection";

function renderAt(path: string): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <CsmSideBar collapsed={false} />
    </MemoryRouter>,
  );
}

describe("CsmSideBar — active section on routes with no owning nav section", () => {
  beforeEach(() => {
    sidebarPropsSpy.mockClear();
    sessionStorage.clear();
  });

  it("defaults to dashboard on a first-ever visit with no remembered section", () => {
    renderAt("/people/user-1");
    expect(lastActiveItem()).toBe("dashboard");
  });

  it("highlights and remembers the owning section for a route that has one", () => {
    renderAt("/admin/users");
    expect(lastActiveItem()).toBe("admin");
    expect(sessionStorage.getItem(LAST_SECTION_KEY)).toBe("admin");
  });

  // Regression test: a full page reload on /people/:id (e.g. a user profile,
  // linked from all over the app and not owned by any nav section) used to
  // always fall back to the hardcoded "dashboard" ref default, even if the
  // user had just come from Settings > Users. It must instead read back
  // whichever section was active before the reload.
  it("resolves to the last section remembered before a reload, not dashboard", () => {
    sessionStorage.setItem(LAST_SECTION_KEY, "admin");
    renderAt("/people/user-1");
    expect(lastActiveItem()).toBe("admin");
  });
});
