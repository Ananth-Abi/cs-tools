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

import SeverityChip from "@components/SeverityChip";

describe("SeverityChip", () => {
  it("renders a real severity as a bold, solid badge", () => {
    render(<SeverityChip severity="S3" />);
    const chip = screen.getByText("S3");
    expect(chip).toBeInTheDocument();
    // Filled variant (MuiChip-filled), not outlined.
    expect(chip.closest(".MuiChip-root")).toHaveClass("MuiChip-filled");
  });

  it("renders 'unset' as a distinct outlined 'Unset' badge, never as S3", () => {
    render(<SeverityChip severity="unset" />);
    expect(screen.getByText("Unset")).toBeInTheDocument();
    expect(screen.queryByText("S3")).not.toBeInTheDocument();
    const chip = screen.getByText("Unset").closest(".MuiChip-root");
    // Outlined, not filled — visually distinct from every real severity chip
    // (including S4, which also uses the "default" grey role but filled).
    expect(chip).toHaveClass("MuiChip-outlined");
    expect(chip).not.toHaveClass("MuiChip-filled");
  });
});
