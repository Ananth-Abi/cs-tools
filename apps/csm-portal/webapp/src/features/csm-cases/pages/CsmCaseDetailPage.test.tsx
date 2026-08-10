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
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

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

// `currentCaseId` is read by the react-router mock below on every render, so
// the test can move the page from one case to another by mutating it and
// calling `rerender`. `vi.hoisted` is required because `vi.mock` factories
// are hoisted above regular module-scope declarations.
const { getCurrentCaseId, setCurrentCaseId } = vi.hoisted(() => {
  let caseId = "case-1";
  return {
    getCurrentCaseId: () => caseId,
    setCurrentCaseId: (next: string) => {
      caseId = next;
    },
  };
});

vi.mock("react-router", () => ({
  useParams: () => ({ caseId: getCurrentCaseId() }),
  useLocation: () => ({
    pathname: `/cases/${getCurrentCaseId()}`,
    search: "",
    hash: "",
    state: undefined,
  }),
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

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
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
function buildCase(id: string): CsmCaseDetail {
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
    description: "<p>Sample description</p>",
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

const useGetCsmCaseDetailMock = vi.fn();
vi.mock("@features/csm-cases/api/useGetCsmCaseDetail", () => ({
  useGetCsmCaseDetail: (id: string | undefined) => useGetCsmCaseDetailMock(id),
}));
useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
  data: id ? buildCase(id) : undefined,
  isLoading: false,
  isError: false,
  error: null,
  refetch: vi.fn(),
  isFetching: false,
  dataUpdatedAt: 0,
}));

vi.mock("@features/csm-cases/api/usePatchCsmCase", () => ({
  usePatchCsmCase: () => ({
    mutate: vi.fn(),
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
vi.mock("@features/csm-cases/components/CaseActionBar", () => ({
  default: () => null,
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
  TagsWidget: () => null,
  WatchersWidget: () => null,
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
import CsmCaseDetailPage from "@features/csm-cases/pages/CsmCaseDetailPage";

describe("CsmCaseDetailPage — time-card edit dialog reset on case change", () => {
  it("stops showing the previous case's edit dialog once the route moves to a new case", () => {
    setCurrentCaseId("case-1");
    const { rerender } = render(<CsmCaseDetailPage />);

    // Switch to the "Time tracking" tab, where CaseTimeCardsPanel (stubbed
    // above) lives, and open the edit dialog for a card on case-1.
    fireEvent.click(screen.getByRole("tab", { name: /time tracking/i }));
    fireEvent.click(
      screen.getByRole("button", { name: /edit fake time card/i }),
    );

    expect(screen.getByTestId("log-time-card-dialog-probe")).toHaveTextContent(
      "editingCardId=card-1 caseId=case-1",
    );

    // Navigate to a different case (same route, only :caseId changes — this
    // page stays mounted, so the render-time reset block is what has to run).
    setCurrentCaseId("case-2");
    rerender(<CsmCaseDetailPage />);

    // Without `setEditTimeCard(null)` in the reset block, the dialog would
    // still be mounted here, showing case-1's card while the rest of the
    // page has already moved on to case-2.
    expect(
      screen.queryByTestId("log-time-card-dialog-probe"),
    ).not.toBeInTheDocument();
  });
});
