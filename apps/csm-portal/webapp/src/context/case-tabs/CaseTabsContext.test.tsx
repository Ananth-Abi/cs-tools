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

import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { useState, type JSX } from "react";
import "@testing-library/jest-dom/vitest";
import {
  CaseTabsProvider,
  useCaseTabsController,
} from "@context/case-tabs/CaseTabsContext";
import { CaseTabsBehaviorProvider } from "@context/case-tabs/CaseTabsBehaviorContext";
import { MAX_OPEN_CASE_TABS } from "@context/case-tabs/caseTabsTypes";

const STORAGE_KEY = "csm.caseTabs.v1";
const BEHAVIOR_STORAGE_KEY = "csm.caseTabs.behavior";

function Probe(): JSX.Element {
  const { tabs, activeTabId, openTab, closeTab } = useCaseTabsController();
  return (
    <div>
      <div data-testid="tab-ids">{tabs.map((t) => t.id).join(",")}</div>
      <div data-testid="active">{activeTabId ?? ""}</div>
      <button onClick={() => openTab("CS1", "case", "/cases/CS1")}>open-cs1</button>
      <button onClick={() => openTab("CS2", "case", "/cases/CS2")}>open-cs2</button>
      <button onClick={() => closeTab(tabs[0]?.id ?? "")}>close-first</button>
    </div>
  );
}

function renderProbe(): ReturnType<typeof render> {
  return render(
    <CaseTabsBehaviorProvider>
      <CaseTabsProvider>
        <Probe />
      </CaseTabsProvider>
    </CaseTabsBehaviorProvider>,
  );
}

describe("CaseTabsProvider", () => {
  beforeEach(() => {
    sessionStorage.clear();
    // These tests exercise the tab MECHANISM itself, not the default
    // behavior mode (that's `CaseTabsBehaviorContext`'s own test file, and
    // the "off by default" regression test) — mode "off" would make every
    // `openTab` call here a no-op, so opt into a mode where tabs actually
    // open.
    localStorage.setItem(BEHAVIOR_STORAGE_KEY, "block");
  });

  it("starts with no tabs when sessionStorage is empty", () => {
    renderProbe();
    expect(screen.getByTestId("tab-ids")).toHaveTextContent("");
    expect(screen.getByTestId("active")).toHaveTextContent("");
  });

  it("opens tabs and tracks the active one", async () => {
    renderProbe();
    await act(async () => screen.getByText("open-cs1").click());
    expect(screen.getByTestId("tab-ids").textContent).toMatch(/^case-tab-/);
    await act(async () => screen.getByText("open-cs2").click());
    const ids = screen.getByTestId("tab-ids").textContent?.split(",") ?? [];
    expect(ids).toHaveLength(2);
    expect(screen.getByTestId("active").textContent).toBe(ids[1]);
  });

  it("persists the open tab set to sessionStorage, not localStorage", async () => {
    renderProbe();
    await act(async () => screen.getByText("open-cs1").click());
    const raw = sessionStorage.getItem(STORAGE_KEY);
    expect(raw).toBeTruthy();
    const persisted = JSON.parse(raw as string);
    expect(persisted.tabs).toHaveLength(1);
    expect(persisted.tabs[0].caseId).toBe("CS1");
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
  });

  it("restores tabs from a prior session's sessionStorage on mount", () => {
    sessionStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        tabs: [{ id: "t1", caseId: "CS9", kind: "case", path: "/cases/CS9" }],
        activeTabId: "t1",
      }),
    );
    renderProbe();
    expect(screen.getByTestId("tab-ids")).toHaveTextContent("t1");
    expect(screen.getByTestId("active")).toHaveTextContent("t1");
  });

  it("refuses a new tab past the cap and returns false from openTab", async () => {
    function CapProbe(): JSX.Element {
      const { tabs, openTab } = useCaseTabsController();
      const [result, setResult] = useState<string>("");
      return (
        <div>
          <div data-testid="count">{tabs.length}</div>
          <div data-testid="overflow-outcome">{result}</div>
          <button
            onClick={() => {
              for (let i = 0; i < MAX_OPEN_CASE_TABS; i++) {
                openTab(`CS${i}`, "case", `/cases/CS${i}`);
              }
            }}
          >
            fill
          </button>
          <button
            onClick={() => {
              const ok = openTab("CS-overflow", "case", "/cases/CS-overflow");
              setResult(ok ? "opened" : "blocked");
            }}
          >
            overflow
          </button>
        </div>
      );
    }
    render(
      <CaseTabsBehaviorProvider>
        <CaseTabsProvider>
          <CapProbe />
        </CaseTabsProvider>
      </CaseTabsBehaviorProvider>,
    );
    await act(async () => screen.getByText("fill").click());
    expect(screen.getByTestId("count")).toHaveTextContent(String(MAX_OPEN_CASE_TABS));
    await act(async () => screen.getByText("overflow").click());
    expect(screen.getByTestId("overflow-outcome")).toHaveTextContent("blocked");
    expect(screen.getByTestId("count")).toHaveTextContent(String(MAX_OPEN_CASE_TABS));
  });
});
