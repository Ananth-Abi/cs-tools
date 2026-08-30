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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { JSX } from "react";
import "@testing-library/jest-dom/vitest";
import { useCaseTabCloseConfirm } from "@features/case-tabs/hooks/useCaseTabCloseConfirm";
import { CaseTabsProvider, useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

const TAB_NO_DRAFT: CaseTabState = {
  id: "t1",
  caseId: "CS1",
  kind: "case",
  path: "/cases/CS1",
  label: "CS1 · First case",
  hasDraft: false,
};
const TAB_WITH_DRAFT: CaseTabState = {
  id: "t2",
  caseId: "CS2",
  kind: "case",
  path: "/cases/CS2",
  label: "CS2 · Second case",
  hasDraft: true,
};

function Harness({ tab }: { tab: CaseTabState }): JSX.Element {
  const { requestClose, dialog } = useCaseTabCloseConfirm();
  const { tabs } = useCaseTabsController();
  return (
    <div>
      <div data-testid="open-count">{tabs.length}</div>
      <button onClick={() => requestClose(tab)}>close</button>
      {dialog}
    </div>
  );
}

describe("useCaseTabCloseConfirm", () => {
  it("closes immediately when the tab has no draft, without confirming", () => {
    render(
      <CaseTabsProvider>
        <Harness tab={TAB_NO_DRAFT} />
      </CaseTabsProvider>,
    );
    fireEvent.click(screen.getByText("close"));
    expect(screen.queryByText("Close this case tab?")).not.toBeInTheDocument();
  });

  it("asks for confirmation when the tab has a draft, and respects Keep tab open", async () => {
    render(
      <CaseTabsProvider>
        <Harness tab={TAB_WITH_DRAFT} />
      </CaseTabsProvider>,
    );
    fireEvent.click(screen.getByText("close"));
    expect(screen.getByText("Close this case tab?")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Keep tab open"));
    // MUI's Dialog unmounts its content after an exit transition rather than
    // synchronously on the click — wait for that instead of asserting
    // immediately.
    await waitFor(() =>
      expect(screen.queryByText("Close this case tab?")).not.toBeInTheDocument(),
    );
  });
});
