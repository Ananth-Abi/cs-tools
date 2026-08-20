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
import type { BeChangeRequestDetail, BePatchChangeRequestPayload } from "@api/backend/types";
import {
  sanitizeRichTextHtml,
  stripHtmlTagsPreservingLineBreaks,
} from "@utils/sanitizeHtml";

// The "Assignment group" picker goes through useSearchGroups, which hits the
// backend client via react-query — stub it out (same approach as
// EditIncidentDialog.test.tsx).
const useSearchGroupsMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));
vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: (...args: unknown[]) => useSearchGroupsMock(...(args as [])),
}));

import EditChangeRequestDialog from "@features/csm-operations/components/EditChangeRequestDialog";

const BASE_CR: BeChangeRequestDetail = {
  id: "chg-1",
  number: "CHG0009988",
  subject: "Upgrade the gateway cluster",
  createdOn: "2026-01-01T00:00:00Z",
  state: "assess",
  type: "normal",
  assignedTeam: { id: "team-1", name: "Platform" },
  hasCustomerApproved: false,
  hasCustomerReviewed: false,
};

/**
 * Render the dialog over `BASE_CR` with the given field overrides, returning the
 * `onSave`/`onClose` spies so a test can assert exactly which fields were
 * submitted — these tests are mostly about the dialog sending *only* changed
 * fields, so the payload passed to `onSave` is the assertion target.
 */
function renderDialog(
  overrides: Partial<BeChangeRequestDetail> = {},
  onSave = vi.fn<(patch: BePatchChangeRequestPayload) => void>(),
): { onSave: typeof onSave; onClose: ReturnType<typeof vi.fn> } {
  const onClose = vi.fn();
  render(
    <EditChangeRequestDialog
      cr={{ ...BASE_CR, ...overrides }}
      isSaving={false}
      onClose={onClose}
      onSave={onSave}
    />,
  );
  return { onSave, onClose };
}

// Queried lazily on each call rather than captured once, because these tests
// toggle the switches and re-read their state after a re-render.

/** The "Customer approved" switch. */
const approvedSwitch = (): HTMLElement => screen.getByLabelText(/customer approved/i);
/** The "Customer reviewed" switch — mutually exclusive with the one above. */
const reviewedSwitch = (): HTMLElement => screen.getByLabelText(/customer reviewed/i);
/** The dialog's Save button. */
const saveButton = (): HTMLElement => screen.getByRole("button", { name: /save/i });

describe("EditChangeRequestDialog — Customer approved / reviewed mutual exclusion", () => {
  it("allows toggling only 'Customer approved' and saves just that field", () => {
    const { onSave } = renderDialog();
    fireEvent.click(approvedSwitch());
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ isCustomerApproved: true });
  });

  it("allows toggling only 'Customer reviewed' and saves just that field", () => {
    const { onSave } = renderDialog();
    fireEvent.click(reviewedSwitch());
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ isCustomerReviewed: true });
  });

  it("locks 'Customer reviewed' once 'Customer approved' has been changed, so both can never be sent together", () => {
    renderDialog();
    fireEvent.click(approvedSwitch());
    expect(reviewedSwitch()).toBeDisabled();
    expect(approvedSwitch()).toBeEnabled();
  });

  it("locks 'Customer approved' once 'Customer reviewed' has been changed", () => {
    renderDialog();
    fireEvent.click(reviewedSwitch());
    expect(approvedSwitch()).toBeDisabled();
    expect(reviewedSwitch()).toBeEnabled();
  });

  it("explains why the other switch is locked", () => {
    renderDialog();
    fireEvent.click(approvedSwitch());
    expect(
      screen.getByText(/can't be changed in the same save/i),
    ).toBeInTheDocument();
  });

  it("unlocks the other switch again once the change is undone", () => {
    renderDialog();
    fireEvent.click(approvedSwitch());
    expect(reviewedSwitch()).toBeDisabled();
    fireEvent.click(approvedSwitch());
    expect(reviewedSwitch()).toBeEnabled();
  });

  it("never builds a patch containing both isCustomerApproved and isCustomerReviewed, even via the disabled control", () => {
    const { onSave } = renderDialog();
    fireEvent.click(approvedSwitch());
    // Attempting to click the now-disabled control is a no-op in the DOM;
    // this documents that expectation rather than relying on it silently.
    fireEvent.click(reviewedSwitch());
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ isCustomerApproved: true });
    const [patch] = onSave.mock.calls[0];
    expect(patch).not.toHaveProperty("isCustomerReviewed");
  });
});

describe("EditChangeRequestDialog — save error surfacing", () => {
  it("renders no error alert by default", () => {
    renderDialog();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("renders the given saveError as a visible alert inside the dialog", () => {
    const onClose = vi.fn();
    render(
      <EditChangeRequestDialog
        cr={BASE_CR}
        isSaving={false}
        saveError="isCustomerApproved, isCustomerReviewed, and requestApproval are mutually exclusive"
        onClose={onClose}
        onSave={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("alert"),
    ).toHaveTextContent(/mutually exclusive/i);
  });
});

// ---------------------------------------------------------------------------
// Fields added so the plan and schedule can actually be entered somewhere:
// rollback plan, test plan, and planned end.
// ---------------------------------------------------------------------------

/** The "Rollback plan" textarea. */
const rollbackPlanField = (): HTMLElement => screen.getByLabelText(/rollback plan/i);
/** The "Test plan" textarea. */
const testPlanField = (): HTMLElement => screen.getByLabelText(/test plan/i);

describe("EditChangeRequestDialog — rollback and test plans", () => {
  it("seeds each plan field from the stored value, with the stored markup stripped", () => {
    renderDialog({
      rollbackPlan: "<p>Restore the previous release.</p>",
      testPlan: "<p>Smoke the gateway health endpoint.</p>",
    });
    expect(rollbackPlanField()).toHaveValue("Restore the previous release.");
    expect(testPlanField()).toHaveValue("Smoke the gateway health endpoint.");
  });

  it("renders both fields empty when the change request has no plans yet", () => {
    renderDialog();
    expect(rollbackPlanField()).toHaveValue("");
    expect(testPlanField()).toHaveValue("");
  });

  it("sends only the rollback plan when only that field was edited", () => {
    const { onSave } = renderDialog();
    fireEvent.change(rollbackPlanField(), {
      target: { value: "Redeploy the previous image tag." },
    });
    fireEvent.click(saveButton());
    // Sent as rich text: the field stores and re-renders HTML.
    expect(onSave).toHaveBeenCalledWith({
      rollbackPlan: "<p>Redeploy the previous image tag.</p>",
    });
  });

  it("sends only the test plan when only that field was edited", () => {
    const { onSave } = renderDialog();
    fireEvent.change(testPlanField(), { target: { value: "Run the regression suite." } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({
      testPlan: "<p>Run the regression suite.</p>",
    });
  });

  it("sends both plans, and nothing else, when both were edited", () => {
    const { onSave } = renderDialog();
    fireEvent.change(rollbackPlanField(), { target: { value: "Roll the image back." } });
    fireEvent.change(testPlanField(), { target: { value: "Run the regression suite." } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({
      rollbackPlan: "<p>Roll the image back.</p>",
      testPlan: "<p>Run the regression suite.</p>",
    });
  });

  it("keeps an untouched plan out of the patch even when the CR already has one stored", () => {
    const { onSave } = renderDialog({
      rollbackPlan: "<p>Restore the previous release.</p>",
      testPlan: "<p>Smoke the gateway health endpoint.</p>",
    });
    fireEvent.click(approvedSwitch());
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ isCustomerApproved: true });
  });

  // An intentional clear must stay an empty string, not become an empty
  // paragraph: `<p></p>` would read as "this plan says nothing" rather than
  // "there is no plan".
  it("treats clearing a stored plan as a real edit and sends the empty value", () => {
    const { onSave } = renderDialog({ rollbackPlan: "<p>Restore the previous release.</p>" });
    fireEvent.change(rollbackPlanField(), { target: { value: "" } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ rollbackPlan: "" });
  });

  it("sends \"\" rather than an empty paragraph for a whitespace-only plan", () => {
    const { onSave } = renderDialog({ rollbackPlan: "<p>Restore the previous release.</p>" });
    fireEvent.change(rollbackPlanField(), { target: { value: "   \n  " } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ rollbackPlan: "" });
  });

  // The other half of the seeding fix: an edited plan is sent as rich text, so
  // the line breaks the engineer typed survive the round trip and a typed `<`
  // or `&` can't drop the rest of the plan when it is rendered back.
  it("escapes and keeps the line breaks of an edited plan", () => {
    const typed = "Stop the rollout if error rate < 1% & rising.\nRedeploy the previous tag.";
    const { onSave } = renderDialog();
    fireEvent.change(rollbackPlanField(), { target: { value: typed } });
    fireEvent.click(saveButton());

    const patch = onSave.mock.calls[0][0];
    expect(patch.rollbackPlan).toBe(
      "<p>Stop the rollout if error rate &lt; 1% &amp; rising.<br />Redeploy the previous tag.</p>",
    );
    // What is stored has to survive the render-side sanitizer and come back
    // as exactly what was typed — this is the same value the dialog will
    // re-seed from next time it opens.
    expect(
      stripHtmlTagsPreservingLineBreaks(sanitizeRichTextHtml(patch.rollbackPlan ?? "")),
    ).toBe(typed);
  });

  it("leaves Save disabled until a plan is actually changed", () => {
    renderDialog({ rollbackPlan: "<p>Restore the previous release.</p>" });
    expect(saveButton()).toBeDisabled();
  });

  // Regression: the strip used to drop block boundaries, so a stored
  // multi-paragraph plan was seeded into the field as one run-on line. Because
  // dirty-tracking is a string compare against the seeded value, saving any
  // unrelated field afterwards could write that flattened text back over the
  // stored content.
  it("keeps paragraph boundaries when seeding a multi-paragraph stored plan", () => {
    renderDialog({
      rollbackPlan: "<p>Stop the rollout.</p><p>Redeploy the previous tag.</p>",
    });
    expect(rollbackPlanField()).toHaveValue(
      "Stop the rollout.\n\nRedeploy the previous tag.",
    );
  });

  it("keeps <br> line breaks and list items on separate lines", () => {
    renderDialog({
      rollbackPlan: "line one<br />line two",
      testPlan: "<ul><li>check health</li><li>check latency</li></ul>",
    });
    expect(rollbackPlanField()).toHaveValue("line one\nline two");
    expect(testPlanField()).toHaveValue("check health\n\ncheck latency");
  });

  it("still keeps a multi-paragraph plan out of the patch when it was not edited", () => {
    const { onSave } = renderDialog({
      rollbackPlan: "<p>Stop the rollout.</p><p>Redeploy the previous tag.</p>",
    });
    fireEvent.click(approvedSwitch());
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ isCustomerApproved: true });
  });
});

describe("EditChangeRequestDialog — planned end must be after planned start", () => {
  it("renders a Planned end picker alongside Planned start", () => {
    renderDialog();
    // The MUI date-time picker renders a segmented group (day/month/year/…),
    // so each picker matches `getByLabelText` several times over, and the
    // outlined field renders its label twice (visible label plus the fieldset
    // legend) — hence `getAllByText`.
    expect(screen.getAllByText("Planned start").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Planned end").length).toBeGreaterThan(0);
  });

  // The patch payload cannot express "remove the planned date" (an omitted key
  // means "leave it alone"), so a clear affordance would look like it worked
  // and then save nothing. Removed from both pickers until the payload can
  // express it.
  it("offers no clear affordance on either picker", () => {
    renderDialog({
      plannedStartOn: "2026-03-01 10:00:00",
      plannedEndOn: "2026-03-01 12:00:00",
    });
    expect(screen.queryByRole("button", { name: /clear/i })).not.toBeInTheDocument();
  });

  it("flags an end that is before the start, and blocks the save", () => {
    renderDialog({
      plannedStartOn: "2026-03-01 10:00:00",
      plannedEndOn: "2026-03-01 09:00:00",
    });
    // Make the form dirty so Save would otherwise be enabled — this proves the
    // date check, not the dirty check, is what disables it.
    fireEvent.click(approvedSwitch());
    expect(
      screen.getByText(/planned end must be after planned start/i),
    ).toBeInTheDocument();
    expect(saveButton()).toBeDisabled();
  });

  it("flags an end equal to the start — a zero-length change window is not a window", () => {
    renderDialog({
      plannedStartOn: "2026-03-01 10:00:00",
      plannedEndOn: "2026-03-01 10:00:00",
    });
    fireEvent.click(approvedSwitch());
    expect(saveButton()).toBeDisabled();
  });

  it("accepts an end after the start", () => {
    renderDialog({
      plannedStartOn: "2026-03-01 10:00:00",
      plannedEndOn: "2026-03-01 12:00:00",
    });
    fireEvent.click(approvedSwitch());
    expect(
      screen.queryByText(/planned end must be after planned start/i),
    ).not.toBeInTheDocument();
    expect(saveButton()).toBeEnabled();
  });

  it("does not flag anything when only one end of the window is set", () => {
    renderDialog({ plannedStartOn: "2026-03-01 10:00:00" });
    fireEvent.click(approvedSwitch());
    expect(
      screen.queryByText(/planned end must be after planned start/i),
    ).not.toBeInTheDocument();
    expect(saveButton()).toBeEnabled();
  });
});
