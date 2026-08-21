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

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { JSX } from "react";
import {
  MemoryRouter,
  Route,
  Routes,
  useLocation,
  useNavigate,
  type NavigateOptions,
  type To,
} from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "@testing-library/jest-dom/vitest";
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";
import type { BeCaseType } from "@api/backend/types";

// `@api/backend/client` -> `useAuthApiClient` -> `@config/apiConfig`, which
// throws at module load when `window.config` isn't set — not present under
// vitest. Same stub other page tests use (e.g. CsmAccountDetailPage.test.tsx).
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

// The backend client reads runtime config at module load, which isn't
// present under vitest. The page imports `BackendApiError` from it directly,
// so stub the module with a real class (so `instanceof` still works) — same
// approach as CsmChangeRequestDetailPage.test.tsx.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

const navigateMock = vi.fn();
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));

vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({
    email: "jane.doe@example.com",
    name: "Jane Doe",
  }),
}));

const showErrorMock = vi.fn();
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));
vi.mock("@features/csm-recent/hooks/useRecentViews", () => ({
  useRecordRecentView: () => vi.fn(),
}));
vi.mock("@utils/useDarkMode", () => ({
  useDarkMode: () => false,
}));

// Builds a minimal but valid CsmCaseDetail whose `id` tracks the currently
// mutated case id, so the page gets past its loading/error gates (the
// `isLoading`/`isError`/`!data` early returns) for whichever case is active.
function buildCase(
  id: string,
  overrides?: { caseType?: BeCaseType; description?: string },
): CsmCaseDetail {
  return {
    id,
    caseNumber: `CS-${id}`,
    subject: "Sample case subject",
    customer: "Acme Corp",
    accountId: "account-1",
    projectId: "project-1",
    projectName: "Acme Project",
    product: "WSO2 Identity Server",
    severity: "S2",
    state: "open",
    assignee: "Unassigned",
    assigneeIsMe: false,
    slaClockType: "resolution",
    minutesToBreach: 120,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    description: overrides?.description ?? "<p>Sample description</p>",
    caseType: overrides?.caseType,
    assignmentGroup: "Support Team",
    customerContext: {
      accountName: "Acme Corp",
      tier: "enterprise",
      region: "US",
      primaryContact: "Jane Doe",
      primaryContactEmail: "jane.doe@example.com",
      accountManager: "John Smith",
      openCases: 1,
    },
    productContext: {
      product: "WSO2 Identity Server",
      version: "7.0",
      deployment: "prod-cluster",
      environment: "prod",
    },
    watchers: [],
    linkedItems: [],
    tags: [],
    timeLogs: [],
    audit: [],
    attachments: [],
    isWatching: false,
  };
}

const WATCHER_ID = "00000000-0000-0000-0000-000000000001";
const NEW_WATCHER_ID = "00000000-0000-0000-0000-000000000002";

const useGetCsmCaseDetailMock = vi.fn();
vi.mock("@features/csm-cases/api/useGetCsmCaseDetail", () => ({
  useGetCsmCaseDetail: (id: string | undefined) => useGetCsmCaseDetailMock(id),
}));
function defaultCaseDetailImpl(id: string | undefined): unknown {
  return {
    data: id ? buildCase(id) : undefined,
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    isFetching: false,
    dataUpdatedAt: 0,
  };
}
useGetCsmCaseDetailMock.mockImplementation(defaultCaseDetailImpl);
// Reset to the shared default after every test — a test that swaps in its own
// implementation (e.g. to set caseType) must not leak that override into
// whichever test runs next.
afterEach(() => {
  useGetCsmCaseDetailMock.mockImplementation(defaultCaseDetailImpl);
});

const patchCaseMutateMock = vi.fn();
vi.mock("@features/csm-cases/api/usePatchCsmCase", () => ({
  usePatchCsmCase: () => ({
    mutate: patchCaseMutateMock,
    mutateAsync: vi.fn(),
    isPending: false,
  }),
  usePatchCsmCaseById: () => vi.fn(),
}));
vi.mock("@features/csm-cases/api/useFindMyOngoingCases", () => ({
  useFindMyOngoingCases: () => vi.fn(),
}));
vi.mock("@features/csm-cases/api/useCsmCaseComments", () => ({
  useGetCsmCaseComments: () => ({
    data: [],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    isFetching: false,
  }),
  usePostCsmCaseComment: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@features/csm-cases/api/useCsmConversationMessages", () => ({
  useGetCsmConversationMessages: () => ({
    data: undefined,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    isFetching: false,
  }),
}));
vi.mock("@features/csm-cases/api/useCsmCaseActivities", () => ({
  useGetCsmCaseActivities: () => ({
    data: undefined,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    isFetching: false,
  }),
}));
vi.mock("@features/csm-cases/api/useCsmCaseAttachments", () => ({
  useGetCsmCaseAttachments: () => ({
    data: undefined,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    isFetching: false,
    dataUpdatedAt: 0,
  }),
  usePostCsmCaseAttachment: () => ({
    mutateAsync: vi.fn(),
    isPending: false,
    isError: false,
    error: null,
  }),
  useDownloadCsmCaseAttachment: () => vi.fn(),
  useDeleteCsmCaseAttachment: () => ({ mutate: vi.fn(), isPending: false }),
  useGetCsmCaseAttachmentContent: () => vi.fn(),
}));
vi.mock("@features/csm-cases/api/useCsmCaseCallRequests", () => ({
  useGetCsmCaseCallRequests: () => ({
    data: undefined,
    refetch: vi.fn(),
    isFetching: false,
  }),
}));
vi.mock("@features/csm-cases/api/useSearchCaseTasks", () => ({
  useSearchCaseTasks: () => ({ data: undefined }),
}));
vi.mock("@features/csm-cases/api/useSearchDeployments", () => ({
  useSearchDeployments: () => ({
    data: undefined,
    isLoading: false,
    refetch: vi.fn(),
    isFetching: false,
  }),
}));
vi.mock("@features/csm-projects/api/useGetProject", () => ({
  useGetProject: () => ({
    data: undefined,
    isLoading: false,
    refetch: vi.fn(),
    isFetching: false,
  }),
}));
vi.mock("@features/csm-cases/api/useGetCsmCaseSlas", () => ({
  useGetCsmCaseSlas: () => ({ data: undefined }),
}));
vi.mock("@features/csm-cases/api/useCreateCaseTask", () => ({
  useCreateCaseTask: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@features/csm-cases/api/useCaseTags", () => ({
  useAddCaseTag: () => ({ mutate: vi.fn(), isPending: false }),
  useRemoveCaseTag: () => ({
    mutate: vi.fn(),
    isPending: false,
    variables: undefined,
  }),
}));
vi.mock("@features/csm-cases/api/useCsmCaseGithubIssue", () => ({
  usePostCaseGithubIssue: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@features/csm-timecards/api/useTimeCards", () => ({
  usePostTimeCard: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateTimeCard: () => ({ mutate: vi.fn(), isPending: false }),
}));

// Simple presentational stubs — none of this test's assertions touch these.
vi.mock("@features/csm-cases/components/CsmCaseCommentInput", () => ({
  default: () => null,
}));
// Probe, not `null`: the change_case_type test below needs a way to open
// ChangeCaseTypeDialog the same way a real user would (via the action bar's
// menu). CaseActionBar's own rendering/gating is covered in
// CaseActionBar.test.tsx.
vi.mock("@features/csm-cases/components/CaseActionBar", () => ({
  default: ({ onAction }: { onAction: (action: { secondary: string }) => void }) => (
    <button type="button" onClick={() => onAction({ secondary: "change_case_type" })}>
      stub open change case type
    </button>
  ),
  canAcknowledge: () => false,
}));
vi.mock("@features/csm-cases/components/AssignEngineerDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/ResolutionDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/ChangeSeverityDialog", () => ({
  default: () => null,
}));
// Probe, not `null`: the dialog's own three-step flow is covered in
// ChangeCaseTypeDialog.test.tsx. Here it just hands the page a finished
// submission so these tests can assert the resulting PATCH body(ies).
vi.mock("@features/csm-cases/components/ChangeCaseTypeDialog", () => ({
  default: ({
    onSubmit,
  }: {
    onSubmit: (
      submission:
        | { targetType: "engagement"; engagementType: string }
        | { targetType: "case"; severity?: string },
    ) => void;
  }) => (
    <>
      <button
        type="button"
        onClick={() => onSubmit({ targetType: "engagement", engagementType: "migration" })}
      >
        stub transfer to engagement
      </button>
      <button
        type="button"
        onClick={() => onSubmit({ targetType: "case", severity: "S2" })}
      >
        stub transfer to case with severity
      </button>
    </>
  ),
}));
vi.mock("@features/csm-cases/components/SetAutocloseHoldDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/EditCaseDetailsDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/LinkIncidentDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/LinkCaseDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/SetFixEtaDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/CreateTaskDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/AddTagDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/ChildCasesWidget", () => ({
  ChildCasesWidget: () => null,
}));
vi.mock("@features/csm-cases/components/LinkedServiceRequestsWidget", () => ({
  LinkedServiceRequestsWidget: () => null,
}));
vi.mock("@features/csm-cases/components/LinkedChangeRequestsWidget", () => ({
  LinkedChangeRequestsWidget: () => null,
}));
vi.mock("@features/csm-cases/components/CreateGithubIssueDialog", () => ({
  CreateGithubIssueDialog: () => null,
}));
vi.mock("@features/csm-cases/components/CaseActivitiesFeed", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/CaseMetaBand", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/CaseDetailWidgets", () => ({
  AttachmentsWidget: () => null,
  CustomerContextWidget: () => null,
  ProductContextWidget: () => null,
  // Stubbed to a probe rather than `null`: the tests below assert only that
  // the page mounts it for a service request and not for a plain case. The
  // widget's own rendering is covered in CaseDetailWidgets.test.tsx.
  RequestDetailsWidget: () => <div data-testid="request-details-widget" />,
  TagsWidget: () => null,
  // Probe, not `null`: the real widget computes the replacement watch list and
  // enforces the per-record-type rules (covered in CaseDetailWidgets.test.tsx).
  // Here it just hands the page a finished list so these tests can see what
  // the page does with it.
  WatchersWidget: ({
    entityKind,
    watchers,
    onReplace,
  }: {
    entityKind: string;
    watchers: Array<{ id: string; name: string }>;
    onReplace?: (nextWatcherIds: string[], action: "add" | "remove") => void;
  }) => (
    <div data-testid="watchers-widget" data-entity-kind={entityKind}>
      <button
        type="button"
        onClick={() =>
          onReplace?.([...watchers.map((w) => w.id), NEW_WATCHER_ID], "add")
        }
      >
        stub add watcher
      </button>
    </div>
  ),
}));
vi.mock("@features/csm-cases/components/CallRequestsWidget", () => ({
  CallRequestsWidget: () => null,
}));
vi.mock("@features/csm-cases/components/TasksWidget", () => ({
  TasksWidget: () => null,
}));
vi.mock("@features/csm-cases/components/CaseSlaTable", () => ({
  CaseSlaTable: () => null,
}));

// The two components at the center of this regression: CaseTimeCardsPanel is
// stubbed to a button that opens the edit dialog for a fake card (mirroring
// the real `onEditTimeCard={setEditTimeCard}` wiring), and LogTimeCardDialog
// is stubbed to a probe that renders the `editingCard` id and `caseId` it was
// actually given — so the test can see whether the dialog is still mounted,
// and against which case, after a route change.
const FAKE_CARD: CsmTimeCard = {
  id: "card-1",
  caseId: "case-1",
  caseNumber: "CS-case-1",
  projectId: "project-1",
  projectName: "Acme Project",
  workDate: "2026-01-01",
  userId: "user-1",
  userName: "Jane Doe",
  state: "submitted",
  billable: true,
  totalMinutes: 60,
} as CsmTimeCard;

vi.mock("@features/csm-timecards/components/CaseTimeCardsPanel", () => ({
  default: ({
    onEditTimeCard,
  }: {
    onEditTimeCard: (card: CsmTimeCard) => void;
  }) => (
    <button type="button" onClick={() => onEditTimeCard(FAKE_CARD)}>
      Edit fake time card
    </button>
  ),
}));
vi.mock("@features/csm-timecards/components/LogTimeCardDialog", () => ({
  default: ({
    caseId,
    editingCard,
  }: {
    caseId: string;
    editingCard?: CsmTimeCard;
  }) => (
    <div data-testid="log-time-card-dialog-probe">
      {`editingCardId=${editingCard?.id ?? "none"} caseId=${caseId}`}
    </div>
  ),
}));

// Imported after the mocks above so the module picks them up.
import { BackendApiError } from "@api/backend/client";
import CsmCaseDetailPage from "@features/csm-cases/pages/CsmCaseDetailPage";

// Renders the real route pathname, so the test can assert the router itself
// actually transitioned (not just that the page re-rendered) — same
// convention as CsmAccountDetailPage.test.tsx's `LocationProbe`.
function LocationProbe(): JSX.Element {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

/** Same idea as `LocationProbe`, surfacing the search string + hash too — for
 * the `?tab=` URL-sync and canonical-redirect tests below. */
function LocationSearchProbe(): JSX.Element {
  const location = useLocation();
  return (
    <div data-testid="location-search-probe">
      {location.pathname}
      {location.search}
      {location.hash}
    </div>
  );
}

// Fires a real `navigate("/cases/case-2")` from inside the router, the same
// way an in-app link (e.g. the sidebar's recent-cases list) would move the
// user from one case to another without unmounting this page — the route
// pattern is identical, only the `:caseId` param changes.
function NavigateToCaseTwoButton(): JSX.Element {
  const navigate = useNavigate();
  return (
    <button type="button" onClick={() => navigate("/cases/case-2")}>
      Go to case 2
    </button>
  );
}

function renderPage(): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/cases/case-1"]}>
        <NavigateToCaseTwoButton />
        <LocationProbe />
        <Routes>
          <Route path="/cases/:caseId" element={<CsmCaseDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const DASHED_ID = "56f49f0a-eb1e-c310-fcf5-f5dabad0cdab";
const DASHLESS_ID = "56f49f0aeb1ec310fcf5f5dabad0cdab";

// `useNavTransition` is mocked module-wide (above) so the rest of this
// file's tests can assert on navigate-call arguments without a real router
// transition. For the dashless-id tests specifically we want to prove the
// router's own location actually changes, not just that the mock was
// invoked — so this bridges the mock to the real `useNavigate` from this
// render tree, and `LocationProbe` (already used elsewhere in this file)
// verifies the resulting location.
function NavigateBridge(): null {
  const navigate = useNavigate();
  navigateMock.mockImplementation((to: To, options?: NavigateOptions) =>
    navigate(to, options),
  );
  return null;
}

function renderPageAtCaseId(initialEntry: string): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <NavigateBridge />
        <LocationProbe />
        <Routes>
          <Route path="/cases/:caseId" element={<CsmCaseDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

// The five real route mount points this page renders under (App.tsx) — all
// must sync `?tab=` the same way, not just the plain /cases/:caseId one. Each
// carries the `caseType` that makes IT the record's own canonical route (see
// `canonicalDetailPath`/`isMisrouted`), so the page renders straight through
// rather than bouncing through the canonical-redirect skeleton first.
const CASE_ROUTE_MOUNTS: Array<{
  name: string;
  path: string;
  caseId: string;
  caseType?: string;
}> = [
  { name: "cases", path: "/cases/:caseId", caseId: "case-1" },
  {
    name: "operations/service-requests",
    path: "/operations/service-requests/:caseId",
    caseId: "case-1",
    caseType: "service_request",
  },
  {
    name: "engagements",
    path: "/engagements/:caseId",
    caseId: "case-1",
    caseType: "engagement",
  },
  {
    name: "security-center/security-reports",
    path: "/security-center/security-reports/:caseId",
    caseId: "case-1",
    caseType: "security_report_analysis",
  },
  {
    name: "announcements",
    path: "/announcements/:caseId",
    caseId: "case-1",
    caseType: "announcement",
  },
];

function renderPageAt(
  initialEntry: string,
  routePath = "/cases/:caseId",
): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <LocationSearchProbe />
        <Routes>
          <Route path={routePath} element={<CsmCaseDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmCaseDetailPage — tab lives in the URL", () => {
  it("defaults to the Activities tab when ?tab= is absent", () => {
    renderPageAt("/cases/case-1");

    expect(screen.getByRole("tab", { name: /activities/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("restores the tab named in the URL on a direct/cold load", () => {
    renderPageAt("/cases/case-1?tab=attachments");

    expect(screen.getByRole("tab", { name: /attachments/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("falls back to Activities for an unrecognised ?tab= value, without crashing or looping", () => {
    renderPageAt("/cases/case-1?tab=not-a-real-tab");

    expect(screen.getByRole("tab", { name: /activities/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByTestId("location-search-probe")).toHaveTextContent(
      "tab=not-a-real-tab",
    );
  });

  it("writes the selected tab to ?tab= when switching tabs, replacing rather than pushing a new history entry", () => {
    renderPageAt("/cases/case-1");

    fireEvent.click(screen.getByRole("tab", { name: /watchers/i }));

    expect(screen.getByTestId("location-search-probe")).toHaveTextContent(
      "tab=watchers",
    );
  });

  it("falls back to Activities for ?tab=tasks, the hidden tab with no rendered <Tab>", () => {
    renderPageAt("/cases/case-1?tab=tasks");

    expect(screen.getByRole("tab", { name: /activities/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    // "tasks" isn't a rendered <Tab> at all (it's hidden from the strip).
    expect(
      screen.queryByRole("tab", { name: /^tasks$/i }),
    ).not.toBeInTheDocument();
  });

  it.each(CASE_ROUTE_MOUNTS)(
    // "Attachments" (not e.g. "SLAs") since it's the one tab every case type
    // in this list renders, including an announcement — which hides
    // related/watchers/sla/time/call-requests entirely (see the TAB_DEFS
    // filter and the isAnnouncement force-to-Activities effect).
    "syncs ?tab= the same way under the $name mount point",
    ({ path, caseId, caseType }) => {
      useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
        data: id ? { ...buildCase(id), caseType } : undefined,
        isLoading: false,
        isError: false,
        error: null,
        refetch: vi.fn(),
        isFetching: false,
        dataUpdatedAt: 0,
      }));

      renderPageAt(`${path.replace(":caseId", caseId)}?tab=attachments`, path);

      expect(
        screen.getByRole("tab", { name: /^attachments$/i }),
      ).toHaveAttribute("aria-selected", "true");

      // Restore the default mock so later tests aren't affected.
      useGetCsmCaseDetailMock.mockImplementation((defaultId: string | undefined) => ({
        data: defaultId ? buildCase(defaultId) : undefined,
        isLoading: false,
        isError: false,
        error: null,
        refetch: vi.fn(),
        isFetching: false,
        dataUpdatedAt: 0,
      }));
    },
  );
});

describe("CsmCaseDetailPage — canonical-route redirect carries ?tab= and #hash forward", () => {
  it("carries the current ?tab= and hash onto the canonical route when a case is opened on a non-canonical one", async () => {
    useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
      data: id ? { ...buildCase(id), caseType: "service_request" } : undefined,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
      dataUpdatedAt: 0,
    }));

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter
          initialEntries={["/cases/case-1?tab=attachments#entry-9"]}
        >
          <NavigateBridge />
          <LocationSearchProbe />
          <Routes>
            <Route path="/cases/:caseId" element={<CsmCaseDetailPage />} />
            <Route
              path="/operations/service-requests/:caseId"
              element={<CsmCaseDetailPage />}
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    // `#entry-9` is a permalink fragment, so once the fixed
    // `permalinkForceRef` logic (see CsmCaseDetailPage.tsx) sees it on this
    // cold load it forces the tab to Activities regardless of the `?tab=`
    // the URL was opened with — same as any other cold load with a fragment,
    // canonical route or not. `setActiveTab`'s own `setSearchParams` call
    // doesn't preserve the hash, which is why it's gone from the settled
    // URL too; that's pre-existing behaviour of every tab switch, not new
    // here.
    await waitFor(() =>
      expect(screen.getByTestId("location-search-probe")).toHaveTextContent(
        "/operations/service-requests/case-1?tab=activities",
      ),
    );

    // Restore the default mock for every test after this one.
    useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
      data: id ? buildCase(id) : undefined,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
      dataUpdatedAt: 0,
    }));
  });
});

describe("CsmCaseDetailPage — permalink fragment forces the Activities tab", () => {
  it("forces Activities on a cold load that already has a permalink fragment, even when ?tab= names a different tab", () => {
    renderPageAt("/cases/case-1?tab=attachments#entry-9");

    expect(screen.getByRole("tab", { name: /activities/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });
});

describe("CsmCaseDetailPage — dashless id normalization", () => {
  it("fetches the case detail with the dashed id and redirects the URL when the route carries a dashless id", async () => {
    useGetCsmCaseDetailMock.mockClear();
    navigateMock.mockClear();

    renderPageAtCaseId(`/cases/${DASHLESS_ID}`);

    // The underlying data-fetch hook must be called with the corrected,
    // dashed id, not the raw dashless one straight off the URL.
    expect(useGetCsmCaseDetailMock).toHaveBeenCalledWith(DASHED_ID);

    // Exercise the real router replacement: the address bar must actually
    // land on the dashed path, not just the mock being called with it.
    await waitFor(() =>
      expect(screen.getByTestId("location-probe")).toHaveTextContent(
        `/cases/${DASHED_ID}`,
      ),
    );
  });

  it("does not redirect or alter an already-dashed id", () => {
    useGetCsmCaseDetailMock.mockClear();
    navigateMock.mockClear();

    renderPageAtCaseId(`/cases/${DASHED_ID}`);

    expect(useGetCsmCaseDetailMock).toHaveBeenCalledWith(DASHED_ID);
    expect(navigateMock).not.toHaveBeenCalled();
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      `/cases/${DASHED_ID}`,
    );
  });
});

describe("CsmCaseDetailPage — time-card edit dialog reset on case change", () => {
  it("stops showing the previous case's edit dialog once the route moves to a new case", () => {
    renderPage();

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/cases/case-1",
    );

    // Switch to the "Time tracking" tab, where CaseTimeCardsPanel (stubbed
    // above) lives, and open the edit dialog for a card on case-1.
    fireEvent.click(screen.getByRole("tab", { name: /time tracking/i }));
    fireEvent.click(
      screen.getByRole("button", { name: /edit fake time card/i }),
    );

    expect(screen.getByTestId("log-time-card-dialog-probe")).toHaveTextContent(
      "editingCardId=card-1 caseId=case-1",
    );

    // Navigate to a different case through a real router transition (same
    // route, only :caseId changes — this page stays mounted, so the
    // render-time reset block is what has to run).
    fireEvent.click(screen.getByRole("button", { name: /go to case 2/i }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/cases/case-2",
    );

    // Without `setEditTimeCard(null)` in the reset block, the dialog would
    // still be mounted here, showing case-1's card while the rest of the
    // page has already moved on to case-2.
    expect(
      screen.queryByTestId("log-time-card-dialog-probe"),
    ).not.toBeInTheDocument();
  });
});

// Announcements have no composer and no real comment thread (see the
// isAnnouncement gate in the page), so the case description never arrives as
// the Activities feed's opening comment the way it does for every other case
// type — it has to be rendered directly. digiops-cs#2800.
function renderCaseDetailPage(
  path: string,
  routePattern: string,
  caseType: BeCaseType | undefined,
  description: string,
): ReturnType<typeof render> {
  useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
    data: id ? buildCase(id, { caseType, description }) : undefined,
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    isFetching: false,
    dataUpdatedAt: 0,
  }));
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path={routePattern} element={<CsmCaseDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmCaseDetailPage — announcement description rendering", () => {
  it("renders the case description below the activity timeline for an announcement", () => {
    renderCaseDetailPage(
      "/announcements/case-1",
      "/announcements/:caseId",
      "announcement",
      "<p>Long advisory content</p>",
    );

    expect(screen.getByText("Long advisory content")).toBeInTheDocument();
    // Not just presence — the description card must render below the
    // Activity timeline, not above or interleaved with it.
    expect(
      screen
        .getByText("Activity timeline")
        .compareDocumentPosition(screen.getByText("Long advisory content")) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("does not render a standalone description card for a non-announcement case", () => {
    // Regular cases already get their description via the comments feed's
    // opening entry (CaseActivitiesFeed, stubbed to null here) — rendering it
    // a second time here would duplicate it.
    renderCaseDetailPage(
      "/cases/case-1",
      "/cases/:caseId",
      "case",
      "<p>Long advisory content</p>",
    );

    expect(screen.queryByText("Long advisory content")).not.toBeInTheDocument();
  });

  it("renders nothing when an announcement has a blank description", () => {
    renderCaseDetailPage(
      "/announcements/case-1",
      "/announcements/:caseId",
      "announcement",
      "",
    );

    expect(screen.queryByText("Description")).not.toBeInTheDocument();
  });
});

describe("CsmCaseDetailPage — Request details card", () => {
  function mockCaseType(caseType?: string): void {
    useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
      data: id ? { ...buildCase(id), caseType } : undefined,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
      dataUpdatedAt: 0,
    }));
  }

  afterEach(() => {
    useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
      data: id ? buildCase(id) : undefined,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
      dataUpdatedAt: 0,
    }));
  });

  it("renders the card in the Details tab of a service request", () => {
    mockCaseType("service_request");

    renderPageAt(
      "/operations/service-requests/case-1?tab=details",
      "/operations/service-requests/:caseId",
    );

    expect(screen.getByTestId("request-details-widget")).toBeInTheDocument();
  });

  it("does not render the card in the Details tab of a plain case", () => {
    mockCaseType(undefined);

    renderPageAt("/cases/case-1?tab=details");

    expect(
      screen.queryByTestId("request-details-widget"),
    ).not.toBeInTheDocument();
  });

  // Not covered here: a service request opened through the generic
  // /cases/:id route. The page's canonical-redirect gate bounces it to
  // /operations/service-requests/:id behind a skeleton before any tab body
  // renders, so `caseType`'s half of the signal can't be observed from that
  // entry point. It is the same `isServiceRequest` value either way — the
  // page computes it once (route || caseType) and this card reuses it.
});

describe("CsmCaseDetailPage — Watchers tab", () => {
  function openWatchers(): void {
    useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
      data: id
        ? {
            ...buildCase(id),
            watchers: [
              {
                id: WATCHER_ID,
                name: "Jane Doe",
                email: "jane.doe@example.com",
              },
            ],
          }
        : undefined,
      isLoading: false,
      isError: false,
    }));
    renderPage();
    fireEvent.click(screen.getByRole("tab", { name: /watchers/i }));
  }

  it("renders the watch list as a case's, so the last-watcher rule applies", () => {
    openWatchers();
    expect(screen.getByTestId("watchers-widget")).toHaveAttribute(
      "data-entity-kind",
      "case",
    );
  });

  it("PATCHes the whole replacement list, keeping the watchers already on it", () => {
    openWatchers();
    fireEvent.click(screen.getByRole("button", { name: /stub add watcher/i }));

    expect(patchCaseMutateMock).toHaveBeenCalledWith(
      { watchList: [WATCHER_ID, NEW_WATCHER_ID] },
      expect.anything(),
    );
  });

  it("surfaces the backend's own message when the write is rejected, leaving the list untouched", () => {
    openWatchers();
    fireEvent.click(screen.getByRole("button", { name: /stub add watcher/i }));

    const handlers = patchCaseMutateMock.mock.calls.at(-1)?.[1] as {
      onError: (err: unknown) => void;
    };
    handlers.onError(
      new BackendApiError(400, 'watchList contains invalid UUID: "not-a-uuid"'),
    );

    expect(showErrorMock).toHaveBeenCalledWith(
      'watchList contains invalid UUID: "not-a-uuid"',
      expect.anything(),
    );
    // No optimistic write happened, so nothing needs unwinding: the widget is
    // still showing the server's list.
    expect(screen.getByTestId("watchers-widget")).toBeInTheDocument();
  });
});

describe("CsmCaseDetailPage — change case type", () => {
  it("PATCHes type and engagementType together for an engagement transfer", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /stub open change case type/i }));
    fireEvent.click(screen.getByRole("button", { name: /stub transfer to engagement/i }));

    expect(patchCaseMutateMock).toHaveBeenCalledWith(
      { type: "engagement", engagementType: "migration" },
      expect.anything(),
    );
  });

  it("follows a case transfer with a separate severity PATCH, since the backend accepts only one field per call", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /stub open change case type/i }));
    fireEvent.click(screen.getByRole("button", { name: /stub transfer to case with severity/i }));

    expect(patchCaseMutateMock).toHaveBeenCalledWith({ type: "case" }, expect.anything());
    // Simulate the type PATCH succeeding, which fires the severity follow-up.
    const typeCallHandlers = patchCaseMutateMock.mock.calls.find(
      ([payload]) => JSON.stringify(payload) === JSON.stringify({ type: "case" }),
    )?.[1] as { onSuccess: () => void };
    act(() => typeCallHandlers.onSuccess());

    expect(patchCaseMutateMock).toHaveBeenCalledWith(
      { severity: "high" },
      expect.anything(),
    );
  });
});
