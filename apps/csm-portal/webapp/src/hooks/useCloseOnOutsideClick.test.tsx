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

import { fireEvent, render } from "@testing-library/react";
import { useRef } from "react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { useCloseOnOutsideClick } from "./useCloseOnOutsideClick";

function Harness({
  active,
  onClose,
}: {
  active: boolean;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement | null>(null);
  useCloseOnOutsideClick(active, ref, "[data-excluded]", onClose);
  return (
    <div>
      <div ref={ref} data-testid="content">
        content
      </div>
      <button type="button" data-testid="outside">
        outside
      </button>
      <button type="button" data-testid="excluded" data-excluded="true">
        excluded
      </button>
    </div>
  );
}

describe("useCloseOnOutsideClick", () => {
  it("calls onClose on a mousedown outside the content ref", () => {
    const onClose = vi.fn();
    const { getByTestId } = render(<Harness active onClose={onClose} />);
    fireEvent.mouseDown(getByTestId("outside"));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("does not call onClose on a mousedown inside the content ref", () => {
    const onClose = vi.fn();
    const { getByTestId } = render(<Harness active onClose={onClose} />);
    fireEvent.mouseDown(getByTestId("content"));
    expect(onClose).not.toHaveBeenCalled();
  });

  it("does not call onClose on a mousedown matching the exclude selector", () => {
    const onClose = vi.fn();
    const { getByTestId } = render(<Harness active onClose={onClose} />);
    fireEvent.mouseDown(getByTestId("excluded"));
    expect(onClose).not.toHaveBeenCalled();
  });

  it("does nothing while inactive", () => {
    const onClose = vi.fn();
    const { getByTestId } = render(<Harness active={false} onClose={onClose} />);
    fireEvent.mouseDown(getByTestId("outside"));
    expect(onClose).not.toHaveBeenCalled();
  });
});
