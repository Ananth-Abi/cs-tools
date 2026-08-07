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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ConversationsTab from "@features/csm-projects/components/ConversationsTab";
import type { BeConversationView } from "@api/backend/types";

// QueryErrorState imports BackendApiError from @api/backend/client, whose
// module reads window.config (via @config/apiConfig) at load time —
// unavailable under vitest. Mock the config so that module evaluates cleanly;
// useBackendApi itself is never called here (useSearchConversations is mocked
// below), so it doesn't need its own mock.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

const mockUseSearchConversations = vi.fn();

vi.mock("@features/csm-projects/api/useSearchConversations", () => ({
  useSearchConversations: (...args: unknown[]) => mockUseSearchConversations(...args),
}));

vi.mock("@features/csm-projects/components/ConversationTranscriptDialog", () => ({
  default: ({
    conversation,
    onClose,
  }: {
    conversation: BeConversationView;
    onClose: () => void;
  }) => (
    <div>
      <div>Transcript for {conversation.id}</div>
      <button onClick={onClose}>Close transcript</button>
    </div>
  ),
}));

function conversation(overrides: Partial<BeConversationView> = {}): BeConversationView {
  return {
    id: "conv-1",
    number: "CONV0000001",
    initialMessage: "Hi, need help",
    messageCount: 4,
    project: { id: "proj-1", name: "Acme" },
    case: null,
    state: "ACTIVE",
    createdOn: "2026-07-01T10:00:00Z",
    createdBy: "Jane Doe",
    ...overrides,
  };
}

describe("ConversationsTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows a loading skeleton while the search is in flight", () => {
    mockUseSearchConversations.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
    });

    render(<ConversationsTab projectId="proj-1" />);

    expect(screen.queryByText("No chat sessions found for this project.")).not.toBeInTheDocument();
  });

  it("shows an error state when the search fails", () => {
    mockUseSearchConversations.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("boom"),
    });

    render(<ConversationsTab projectId="proj-1" />);

    expect(screen.getByText("boom")).toBeInTheDocument();
  });

  it("shows an empty state when the project has no conversations", () => {
    mockUseSearchConversations.mockReturnValue({
      data: { conversations: [], total: 0 },
      isLoading: false,
      isError: false,
      error: null,
    });

    render(<ConversationsTab projectId="proj-1" />);

    expect(screen.getByText("No chat sessions found for this project.")).toBeInTheDocument();
  });

  it("lists conversations and opens the transcript dialog when a row is clicked", () => {
    mockUseSearchConversations.mockReturnValue({
      data: { conversations: [conversation()], total: 1 },
      isLoading: false,
      isError: false,
      error: null,
    });

    render(<ConversationsTab projectId="proj-1" />);

    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.queryByText("Transcript for conv-1")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("Jane Doe"));

    expect(screen.getByText("Transcript for conv-1")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Close transcript"));

    expect(screen.queryByText("Transcript for conv-1")).not.toBeInTheDocument();
  });

  it("renders a dash for a conversation with no resolved state", () => {
    mockUseSearchConversations.mockReturnValue({
      data: { conversations: [conversation({ state: null })], total: 1 },
      isLoading: false,
      isError: false,
      error: null,
    });

    render(<ConversationsTab projectId="proj-1" />);

    expect(screen.queryByText("Active")).not.toBeInTheDocument();
    expect(screen.queryByText("Resolved")).not.toBeInTheDocument();
  });
});
