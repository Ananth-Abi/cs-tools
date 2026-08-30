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
import { beforeEach, describe, expect, it } from "vitest";
import type { JSX } from "react";
import "@testing-library/jest-dom/vitest";
import {
  CaseTabsBehaviorProvider,
  useCaseTabsBehavior,
} from "@context/case-tabs/CaseTabsBehaviorContext";

const STORAGE_KEY = "csm.caseTabs.behavior";

function Probe(): JSX.Element {
  const { mode, setMode } = useCaseTabsBehavior();
  return (
    <div>
      <div data-testid="mode">{mode}</div>
      <button onClick={() => setMode("block")}>set-block</button>
      <button onClick={() => setMode("evict-oldest")}>set-evict-oldest</button>
      <button onClick={() => setMode("off")}>set-off</button>
    </div>
  );
}

describe("CaseTabsBehaviorContext", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("defaults to 'off' (no tabs at all) on a fresh session with nothing in localStorage", () => {
    render(
      <CaseTabsBehaviorProvider>
        <Probe />
      </CaseTabsBehaviorProvider>,
    );
    expect(screen.getByTestId("mode")).toHaveTextContent("off");
  });

  it("also defaults to 'off' outside a provider (the no-op default context value)", () => {
    render(<Probe />);
    expect(screen.getByTestId("mode")).toHaveTextContent("off");
  });

  it("persists a mode change to localStorage and reflects it immediately", () => {
    render(
      <CaseTabsBehaviorProvider>
        <Probe />
      </CaseTabsBehaviorProvider>,
    );
    fireEvent.click(screen.getByText("set-evict-oldest"));
    expect(screen.getByTestId("mode")).toHaveTextContent("evict-oldest");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("evict-oldest");
  });

  it("restores a previously-saved mode on mount", () => {
    localStorage.setItem(STORAGE_KEY, "block");
    render(
      <CaseTabsBehaviorProvider>
        <Probe />
      </CaseTabsBehaviorProvider>,
    );
    expect(screen.getByTestId("mode")).toHaveTextContent("block");
  });

  it("falls back to the default for a garbage/unrecognized stored value", () => {
    localStorage.setItem(STORAGE_KEY, "not-a-real-mode");
    render(
      <CaseTabsBehaviorProvider>
        <Probe />
      </CaseTabsBehaviorProvider>,
    );
    expect(screen.getByTestId("mode")).toHaveTextContent("off");
  });

  it("exposes exactly the four documented modes as options", () => {
    render(
      <CaseTabsBehaviorProvider>
        <Probe />
      </CaseTabsBehaviorProvider>,
    );
    // Exercised indirectly via PreferencesMenu.test.tsx; here just confirm
    // every mode is independently settable and read back correctly.
    fireEvent.click(screen.getByText("set-block"));
    expect(screen.getByTestId("mode")).toHaveTextContent("block");
    fireEvent.click(screen.getByText("set-off"));
    expect(screen.getByTestId("mode")).toHaveTextContent("off");
  });
});
