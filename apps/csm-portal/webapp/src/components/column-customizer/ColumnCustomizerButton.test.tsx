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
import ColumnCustomizerButton from "@components/column-customizer/ColumnCustomizerButton";
import type { ColumnOption } from "@hooks/useColumnPreferences";

const COLUMNS: ColumnOption[] = [
  { id: "a", label: "Column A" },
  { id: "b", label: "Column B" },
  { id: "c", label: "Column C" },
];

function setup(visible: string[] = ["a", "b", "c"]) {
  const onToggle = vi.fn();
  const onMove = vi.fn();
  const onReset = vi.fn();
  render(
    <ColumnCustomizerButton
      allColumns={COLUMNS}
      isVisible={(id) => visible.includes(id)}
      onToggle={onToggle}
      onMove={onMove}
      onReset={onReset}
    />,
  );
  return { onToggle, onMove, onReset };
}

describe("ColumnCustomizerButton", () => {
  it("opens the popover listing every known column on trigger click", () => {
    setup();
    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));

    expect(screen.getByText("Column A")).toBeInTheDocument();
    expect(screen.getByText("Column B")).toBeInTheDocument();
    expect(screen.getByText("Column C")).toBeInTheDocument();
  });

  it("calls onToggle when a column row is clicked", () => {
    const { onToggle } = setup();
    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));

    fireEvent.click(screen.getByText("Column B"));
    expect(onToggle).toHaveBeenCalledWith("b");
  });

  it("calls onMove when a reorder arrow is clicked, without also toggling that row", () => {
    const { onMove, onToggle } = setup();
    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));

    fireEvent.click(screen.getByRole("button", { name: "Move Column B up" }));
    expect(onMove).toHaveBeenCalledWith("b", "up");
    expect(onToggle).not.toHaveBeenCalled();
  });

  it("disables the up arrow for the first column and the down arrow for the last", () => {
    setup();
    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));

    expect(screen.getByRole("button", { name: "Move Column A up" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Move Column C down" })).toBeDisabled();
  });

  it("disables unchecking the last remaining visible column", () => {
    setup(["a"]);
    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));

    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes[0]).toBeDisabled();
  });

  it("calls onReset when Reset to default is clicked", () => {
    const { onReset } = setup();
    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));

    fireEvent.click(screen.getByText("Reset to default"));
    expect(onReset).toHaveBeenCalled();
  });
});
