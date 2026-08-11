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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { useState } from "react";
import WidgetFilterConditionEditor from "@features/csm-admin/dashboards/components/WidgetFilterConditionEditor";
import type { FilterCondition } from "@features/csm-admin/dashboards/utils/widgetQueryConditions";

function Harness({
  initial,
  resourceType = "case",
  onChangeSpy,
}: {
  initial: FilterCondition[];
  resourceType?: "case" | "incident";
  onChangeSpy?: (next: FilterCondition[]) => void;
}) {
  const [conditions, setConditions] = useState(initial);
  return (
    <WidgetFilterConditionEditor
      resourceType={resourceType}
      conditions={conditions}
      onChange={(next) => {
        setConditions(next);
        onChangeSpy?.(next);
      }}
    />
  );
}

describe("WidgetFilterConditionEditor", () => {
  it("shows an empty-filters message and no rows when there are no conditions", () => {
    render(<Harness initial={[]} />);
    expect(screen.getByText(/matches every case record/i)).toBeInTheDocument();
    expect(screen.queryByLabelText("Filter field")).not.toBeInTheDocument();
  });

  it("adds a new empty row when 'Add filter' is clicked", () => {
    render(<Harness initial={[]} />);
    fireEvent.click(screen.getByRole("button", { name: /add filter/i }));
    expect(screen.getAllByLabelText("Filter field")).toHaveLength(1);
  });

  it("removes a row when its own remove button is clicked", () => {
    render(
      <Harness
        initial={[
          { field: "state", op: "in", values: ["open"] },
          { field: "severity", op: "in", values: ["critical"] },
        ]}
      />,
    );
    expect(screen.getAllByLabelText("Filter field")).toHaveLength(2);

    fireEvent.click(screen.getAllByRole("button", { name: "Remove filter" })[0]);

    expect(screen.getAllByLabelText("Filter field")).toHaveLength(1);
    expect(screen.getByDisplayValue("severity")).toBeInTheDocument();
  });

  it("hides the value input for a value-less op (isEmpty)", () => {
    render(<Harness initial={[{ field: "escalation", op: "isEmpty", values: [] }]} />);
    expect(screen.queryByLabelText("Filter value")).not.toBeInTheDocument();
  });

  it("shows the value input for a value-carrying op", () => {
    render(<Harness initial={[{ field: "state", op: "in", values: ["open"] }]} />);
    expect(screen.getByLabelText("Filter value")).toBeInTheDocument();
  });

  it("calls onChange with an updated op when the operator select changes", () => {
    const onChangeSpy = vi.fn();
    render(
      <Harness
        initial={[{ field: "escalation", op: "isEmpty", values: [] }]}
        onChangeSpy={onChangeSpy}
      />,
    );
    // MUI Select renders its current value in a `role="combobox"` element —
    // open it and pick the option, rather than firing a raw DOM `change`
    // (there's no native <select> element setter here to fire it against).
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Operator" }));
    fireEvent.click(screen.getByRole("option", { name: "is not empty" }));
    expect(onChangeSpy).toHaveBeenCalledWith([{ field: "escalation", op: "isNotEmpty", values: [] }]);
  });
});
