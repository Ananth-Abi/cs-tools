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
import EditDeployedProductDialog from "@features/csm-projects/components/EditDeployedProductDialog";
import type { BeDeployedProduct } from "@api/backend/types";

const DEPLOYED_PRODUCT: BeDeployedProduct = {
  id: "dp-1",
  product: { id: "prod-1", name: "API Manager" },
  cores: 4,
  tps: 100,
  updates: [{ updateLevel: 1, date: "2026-01-01", details: "Initial rollout" }],
};

function renderDialog(overrides?: Partial<BeDeployedProduct>) {
  const onSave = vi.fn();
  const onClose = vi.fn();
  render(
    <EditDeployedProductDialog
      deployedProduct={{ ...DEPLOYED_PRODUCT, ...overrides }}
      isSaving={false}
      onClose={onClose}
      onSave={onSave}
    />,
  );
  return { onSave, onClose };
}

const saveButton = (): HTMLElement =>
  screen.getByRole("button", { name: /save changes/i });

describe("EditDeployedProductDialog", () => {
  it("disables Save until a field changes", () => {
    renderDialog();
    expect(saveButton()).toBeDisabled();
  });

  it("renders numeric cores/tps directly (no re-parse of strings)", () => {
    renderDialog();
    expect(screen.getByLabelText(/cores/i)).toHaveValue(4);
    expect(screen.getByLabelText(/tps/i)).toHaveValue(100);
  });

  it("sends only the changed cores as a number", () => {
    const { onSave } = renderDialog();
    fireEvent.change(screen.getByLabelText(/cores/i), { target: { value: "8" } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ cores: 8 });
  });

  it("sends null when cores is cleared", () => {
    const { onSave } = renderDialog();
    fireEvent.change(screen.getByLabelText(/cores/i), { target: { value: "" } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ cores: null });
  });

  it("switches to the Update History tab and lists existing entries", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("tab", { name: /update history/i }));
    expect(screen.getByText("Initial rollout")).toBeInTheDocument();
    expect(screen.getByText("2026-01-01")).toBeInTheDocument();
  });

  it("adds a new update-history row and saves the whole array on Save changes", () => {
    const { onSave } = renderDialog();
    fireEvent.click(screen.getByRole("tab", { name: /update history/i }));

    fireEvent.change(screen.getByLabelText(/^update level$/i), { target: { value: "2" } });
    fireEvent.change(screen.getByLabelText(/^date$/i), { target: { value: "2026-02-01" } });
    fireEvent.click(screen.getByRole("button", { name: /^add$/i }));

    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({
      updates: [
        { updateLevel: 1, date: "2026-01-01", details: "Initial rollout" },
        { updateLevel: 2, date: "2026-02-01", details: undefined },
      ],
    });
  });

  it("deletes an update-history row and saves the shrunk array", () => {
    const { onSave } = renderDialog();
    fireEvent.click(screen.getByRole("tab", { name: /update history/i }));

    fireEvent.click(screen.getByRole("button", { name: /delete update level 1/i }));
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ updates: [] });
  });
});
