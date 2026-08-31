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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactNode } from "react";
import CasesFilterBar, {
  type CasesFilters,
} from "@features/csm-cases/components/CasesFilterBar";
import { DEFAULT_CASES_FILTERS } from "@features/csm-cases/utils/casesFiltersUrl";
import { useSearchTags } from "@features/csm-cases/api/useSearchTags";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock, get: vi.fn() }),
}));

vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

// Mocked directly (same approach as AddTagDialog.test.tsx) so tests don't
// have to drive the real 300ms debounce in `AsyncTagMultiSelect` (rendered
// only for an Advanced-mode `tag` row now — Tags is Advanced-only, see the
// mode-toggle tests below).
vi.mock("@features/csm-cases/api/useSearchTags", () => ({
  useSearchTags: vi.fn(),
}));
const mockedUseSearchTags = vi.mocked(useSearchTags);
function mockTagSearchResult(
  overrides: Partial<ReturnType<typeof useSearchTags>>,
): void {
  mockedUseSearchTags.mockReturnValue({
    data: [],
    isFetching: false,
    isError: false,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchTags>);
}
// A default mock return value, set once here at file scope, so any test that
// does land in Advanced mode with a `tag` row visible doesn't crash for want
// of a mock.
beforeEach(() => {
  mockTagSearchResult({});
});

function renderBar(
  filters: CasesFilters,
  onChange = vi.fn(),
  extraProps: Partial<Parameters<typeof CasesFilterBar>[0]> = {},
): { onChange: ReturnType<typeof vi.fn> } {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <CasesFilterBar
        filters={filters}
        onChange={onChange}
        onReset={() => {}}
        isFiltersOpen
        onFiltersToggle={() => {}}
        availableAssigneeUsers={[]}
        availableProjects={[]}
        {...extraProps}
      />
    </QueryClientProvider> as ReactNode,
  );
  return { onChange };
}

describe("CasesFilterBar — active-filter chips for URL-only fields", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ teams: [] });
  });

  it("renders no chips when nothing is active", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    expect(screen.queryByText(/SLA/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Escalat/)).not.toBeInTheDocument();
    expect(screen.queryByText(/CS team:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Tag:/)).not.toBeInTheDocument();
  });

  it("renders one chip per URL-only filter and each is independently removable", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 80,
      hasEscalation: true,
      createdOnGte: "2026-07-27",
    });

    expect(screen.getByText("SLA ≥ 80%")).toBeInTheDocument();
    expect(screen.getByText("Escalated")).toBeInTheDocument();
    expect(screen.getByText(/Created after/)).toBeInTheDocument();

    // Removing the SLA chip clears only slaElapsedPctGte, nothing else.
    const slaChip = screen.getByText("SLA ≥ 80%");
    const deleteIcon = slaChip.parentElement?.querySelector(
      '[data-testid="CancelIcon"], svg',
    );
    fireEvent.click(deleteIcon ?? slaChip);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        slaElapsedPctGte: null,
        hasEscalation: true,
        createdOnGte: "2026-07-27",
      }),
    );
  });

  it("renders a date-only bound as the same local calendar date, not shifted by UTC parsing", () => {
    renderBar({
      ...DEFAULT_CASES_FILTERS,
      createdOnGte: "2026-07-27",
    });

    // A bare YYYY-MM-DD bound must render as that same calendar date
    // regardless of the runner's local timezone offset from UTC — pinning
    // this to a fixed local Date (not `new Date("2026-07-27")`, which is
    // parsed as UTC midnight and can roll back a day) is what makes this
    // assertion timezone-safe.
    const expected = new Date(2026, 6, 27).toLocaleDateString();
    expect(
      screen.getByText(`Created after ${expected}`),
    ).toBeInTheDocument();
  });

  it("both SLA bounds render and clear independently", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 80,
      slaElapsedPctLte: 100,
    });

    expect(screen.getByText("SLA ≥ 80%")).toBeInTheDocument();
    expect(screen.getByText("SLA ≤ 100%")).toBeInTheDocument();

    const chip = screen.getByText("SLA ≤ 100%");
    const deleteIcon = chip.parentElement?.querySelector("svg");
    fireEvent.click(deleteIcon ?? chip);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ slaElapsedPctGte: 80, slaElapsedPctLte: null }),
    );
  });
});


describe("CasesFilterBar — removed bar controls fall back to chips", () => {
  beforeEach(() => {
    postMock.mockReset();
  });


  /**
   * `tags`/`excludeTags` are Advanced-mode-only now (see the mode toggle in
   * `CasesFilterBar.tsx`): any non-empty value forces Advanced mode on
   * mount, where the Tag row itself (in the unified builder) is the visible/
   * removable UI — deliberately still NOT chipped here, to avoid showing the
   * same selection twice.
   */
  it("does not render a chip for tags — Advanced mode's own Tag row is the visible/removable UI", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, tags: ["micro-gw"] });
    expect(screen.queryByText(/^Tag: /)).not.toBeInTheDocument();
  });

  it("does not render a chip for excludeTags — Advanced mode's own Tag row is the visible/removable UI", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, excludeTags: ["s_dip"] });
    expect(screen.queryByText(/^Excluding tag:/)).not.toBeInTheDocument();
  });

  it("does not render a chip for csTeams — it has its own 'Team' bar control", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, csTeams: ["g1"] });
    expect(screen.queryByText(/^CS team:/)).not.toBeInTheDocument();
  });

  // `excludeStates` now has its own "State" bar control (the tri-state
  // `TriStateMultiSelectField`, digiops-cs#2907 follow-up), same as
  // `excludeTags` above — no second, redundant chip.
  it("does not render a chip for excludeStates — it has its own 'State' bar control now", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, states: ["open"], excludeStates: ["closed"] });
    expect(screen.queryByText(/^Excluding state:/)).not.toBeInTheDocument();
  });

  it("does not render a chip for onboardingStatuses — it has its own 'Onboarding status' bar control", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, onboardingStatuses: ["Completed"] });
    expect(screen.queryByText(/^Onboarding:/)).not.toBeInTheDocument();
  });
});

describe("CasesFilterBar — 'CRE Team' control (replaces the removed 'Work state' one)", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({
      teams: [
        { id: "abt-1", name: "ABT One", family: "cre-abt", creGroupId: "g-1" },
        { id: "abt-2", name: "ABT Two", family: "cre-abt", creGroupId: "g-2" },
        // No creGroupId configured -- must not appear as a selectable option.
        { id: "abt-3", name: "ABT Three", family: "cre-abt" },
        // Has a creGroupId but a non-`cre-abt` family -- must not appear either.
        { id: "abt-4", name: "ABT Four", family: "cre", creGroupId: "g-4" },
      ],
    });
  });

  it("renders team display names as options, backed by creGroupId (what the filter actually matches on)", async () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "CRE Team" }));
    expect(await screen.findByRole("option", { name: "ABT One" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "ABT Two" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "ABT Three" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "ABT Four" })).not.toBeInTheDocument();
  });

  it("selecting a team sets csTeams to its creGroupId, not its registry id", async () => {
    const { onChange } = renderBar({ ...DEFAULT_CASES_FILTERS });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "CRE Team" }));
    fireEvent.click(await screen.findByRole("option", { name: "ABT One" }));

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ csTeams: ["g-1"] }));
  });
});

describe("CasesFilterBar — 'State' control (tri-state include/exclude, digiops-cs#2907 follow-up)", () => {
  it("clicking an unselected state once includes it", async () => {
    const { onChange } = renderBar({ ...DEFAULT_CASES_FILTERS });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "State" }));
    fireEvent.click(await screen.findByRole("option", { name: "Closed" }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ states: ["closed"], excludeStates: [] }),
    );
  });

  it("clicking an included state a second time moves it to excluded", async () => {
    const { onChange } = renderBar({ ...DEFAULT_CASES_FILTERS, states: ["closed"] });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "State" }));
    fireEvent.click(await screen.findByRole("option", { name: "Closed" }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ states: [], excludeStates: ["closed"] }),
    );
  });

  it("clicking an excluded state a third time clears it back to unselected", async () => {
    const { onChange } = renderBar({ ...DEFAULT_CASES_FILTERS, excludeStates: ["closed"] });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "State" }));
    fireEvent.click(await screen.findByRole("option", { name: "Closed" }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ states: [], excludeStates: [] }),
    );
  });
});

// Tags moved out of Simple mode entirely (Advanced-only now, see
// `CasesFilterBar.tsx`'s mode toggle) -- the old tri-state cycling control
// (`TagsMultiSelect`) tested here is no longer rendered in the Simple grid
// at all. Its replacement — a plain `tag` `in`/`notIn` row in the unified
// Advanced-mode builder, backed by `AsyncTagMultiSelect` — is covered by
// `filterFieldAdapters.test.ts`'s adapter round-trip tests and
// `advancedFilters.test.ts`'s catalogue tests instead.
describe("CasesFilterBar — Tags is Advanced-only", () => {
  it("does not render a 'Tags' control in the Simple grid", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    expect(screen.queryByRole("combobox", { name: "Tags" })).not.toBeInTheDocument();
  });

  it("any active tags/excludeTags filter forces the bar into Advanced mode on mount", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, tags: ["micro-gw"] });
    // In Advanced mode, the Simple-only "State" combobox is gone and the
    // "Advanced filters" row builder is showing instead.
    expect(screen.queryByRole("combobox", { name: "State" })).not.toBeInTheDocument();
    expect(screen.getByText("Advanced filters")).toBeInTheDocument();
  });
});

// Regression: reported live — with every control the same width, a project
// name of any real length ellipsized almost immediately, and the control sat
// mid-row rather than having room to grow. Moved to the end of the grid and
// widened.
describe("CasesFilterBar — 'Project' control is last and wider than its siblings", () => {
  it("renders after every other filter control in document order", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });

    const project = screen.getByRole("combobox", { name: "Project" });
    const onboarding = screen.getByRole("combobox", { name: "Onboarding status" });

    expect(
      project.compareDocumentPosition(onboarding) & Node.DOCUMENT_POSITION_PRECEDING,
    ).toBeTruthy();
  });
});

describe("CasesFilterBar — 'Onboarding status' control", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("offers all 4 fixed projectOnboardingStatus choices", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Onboarding status" }));
    expect(screen.getByRole("option", { name: "In progress" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Not started" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Completed" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Not applicable" })).toBeInTheDocument();
  });

  // The control edits the real `onboardingStatuses` (`in`) field directly —
  // there is no separate exclude field/URL param for this control to write
  // to. A dashboard widget's `projectOnboardingStatus notIn` filter is
  // folded into this same field as its complement at the translation
  // boundary (`translateCaseDashboardFilters`), specifically so this bar
  // control's URL param never collides with a second, exclude-flavored one.
  it("selecting a value sets onboardingStatuses to its raw backend token", () => {
    const { onChange } = renderBar({ ...DEFAULT_CASES_FILTERS });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Onboarding status" }));
    fireEvent.click(screen.getByRole("option", { name: "In progress" }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ onboardingStatuses: ["In-Progress"] }),
    );
  });

  it("adds to any value already set rather than replacing it", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      onboardingStatuses: ["Not-Applicable"],
    });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Onboarding status" }));
    fireEvent.click(screen.getByRole("option", { name: "In progress" }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ onboardingStatuses: ["Not-Applicable", "In-Progress"] }),
    );
  });
});

describe("CasesFilterBar — work state has no bar control, only a chip", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("no longer renders a Work state bar control", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, states: ["work_in_progress"] });
    expect(screen.queryByRole("combobox", { name: "Work state" })).not.toBeInTheDocument();
  });

  it("renders a removable chip when workStates is set (e.g. from a saved view or dashboard click-through)", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      states: ["work_in_progress"],
      workStates: ["ongoing"],
    });

    expect(screen.getByText("Work state: Ongoing")).toBeInTheDocument();

    fireEvent.click(
      screen.getByText("Work state: Ongoing").closest(".MuiChip-root")!.querySelector("svg")!,
    );
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ workStates: [] }));
  });

  // The underlying invariant (workStates only means something when
  // work_in_progress is the *sole* selected state) still matters even
  // without a bar control to disable -- a stale workStates value from a
  // saved view/URL must not survive the State control widening past
  // work_in_progress alone.
  it("clears workStates when a second state is added alongside work_in_progress", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      states: ["work_in_progress"],
      workStates: ["ongoing"],
    });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "State" }));
    fireEvent.click(screen.getByRole("option", { name: "Open" }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        states: ["work_in_progress", "open"],
        workStates: [],
      }),
    );
  });
});

describe("CasesFilterBar — saved views reordering", () => {
  beforeEach(() => {
    postMock.mockReset();
    localStorage.clear();
    localStorage.setItem(
      "csm.savedFilters.v1",
      JSON.stringify([
        { name: "First", qs: "states=open" },
        { name: "Second", qs: "states=closed" },
      ]),
    );
  });

  function openSavedViewsMenu(): void {
    fireEvent.click(screen.getByRole("button", { name: /Saved views/ }));
  }

  it("renders move up/down buttons for saved views; the Suggested section is gone", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    openSavedViewsMenu();

    // The built-in Suggested section has been removed entirely.
    expect(screen.queryByText("Suggested")).not.toBeInTheDocument();
    expect(screen.queryByText("S0/S1 active")).not.toBeInTheDocument();

    // Saved (user) views get reorder controls.
    expect(
      screen.getByRole("button", { name: "Move saved view First down" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Move saved view Second up" }),
    ).toBeInTheDocument();
  });

  it("disables (or omits an enabled) up-arrow on the first item", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    openSavedViewsMenu();

    expect(
      screen.getByRole("button", { name: "Move saved view First up" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Move saved view Second down" }),
    ).toBeDisabled();
  });

  it("clicking move-down on the first saved view reorders the list without applying it", () => {
    const { onChange } = renderBar({ ...DEFAULT_CASES_FILTERS });
    openSavedViewsMenu();

    fireEvent.click(
      screen.getByRole("button", { name: "Move saved view First down" }),
    );

    // Reordering must not also apply/select the view.
    expect(onChange).not.toHaveBeenCalled();

    // The persisted order flips, reflected back through the reactive hook —
    // "Second" now moves up-button-enabled into the first slot.
    expect(
      screen.getByRole("button", { name: "Move saved view Second up" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Move saved view First up" }),
    ).not.toBeDisabled();
  });
});

describe("CasesFilterBar — case-type control label", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("defaults the case-type control's label to \"Case type\"", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    expect(screen.getByLabelText("Case type")).toBeInTheDocument();
  });

  it("renders a caller-supplied typeFilterLabel instead (e.g. a project's mixed work-items view)", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS }, vi.fn(), {
      typeFilterLabel: "Work item type",
    });
    expect(screen.getByLabelText("Work item type")).toBeInTheDocument();
    expect(screen.queryByLabelText("Case type")).not.toBeInTheDocument();
  });

  it("hides the control entirely when hideTypeFilter is set, regardless of typeFilterLabel", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS }, vi.fn(), {
      hideTypeFilter: true,
      typeFilterLabel: "Work item type",
    });
    expect(screen.queryByLabelText("Work item type")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Case type")).not.toBeInTheDocument();
  });
});
