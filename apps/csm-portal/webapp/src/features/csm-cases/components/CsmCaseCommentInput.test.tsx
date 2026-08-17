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

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as CaseActivitiesFeed.test.tsx). Pulled in
// transitively via CsmUploadAttachmentModal -> useCsmCaseAttachments, just
// for its MAX_ATTACHMENT_SIZE_BYTES constant — the mock only exists to keep
// that import chain from throwing on missing runtime config.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn() }),
}));

import CsmCaseCommentInput from "@features/csm-cases/components/CsmCaseCommentInput";

// This component isn't under test here; stub it to a plain textarea (same
// technique EditCaseDetailsDialog.test.tsx uses for the same dependency).
// Also surfaces `attachments.length` as text so drag-and-drop tests below
// can assert on it without needing the real editor's own attachment-chip UI.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({
    value,
    onChange,
    attachments,
  }: {
    value: string;
    onChange: (v: string) => void;
    attachments: File[];
  }) => (
    <div data-testid="editor-stub">
      <textarea
        aria-label="comment-editor"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      <span data-testid="attachment-count">{attachments.length}</span>
    </div>
  ),
}));

const PAUSED_REASON =
  "This case is paused — customer replies are disabled. Resume work to reply to the customer.";
const NOT_STARTED_REASON =
  "Customer replies are disabled unless the case is actively in progress.";

describe("CsmCaseCommentInput — resume-work quick-fix", () => {
  it("shows the inline resume link when the only lock reason is paused work, and calls onResumeWork", () => {
    const onResumeWork = vi.fn();
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={PAUSED_REASON}
        canResumeToUnlockPublicReply
        onResumeWork={onResumeWork}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Resume work" }));
    expect(onResumeWork).toHaveBeenCalledTimes(1);
  });

  it("doesn't also repeat the raw lock reason in the send-row status line once the quick-fix covers it", () => {
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={PAUSED_REASON}
        canResumeToUnlockPublicReply
        onResumeWork={vi.fn()}
      />,
    );

    // The quick-fix's own sentence is present once...
    expect(
      screen.getByText("Only resumed work can send public replies to the customer.", {
        exact: false,
      }),
    ).toBeInTheDocument();
    // ...and the send-row status line falls back to the normal hint instead
    // of repeating the raw backend reason a second time.
    expect(screen.queryByText(PAUSED_REASON)).not.toBeInTheDocument();
    expect(screen.getByText("Ctrl/Cmd + Enter to send.")).toBeInTheDocument();
  });

  it("does not show the resume link when the case hasn't started yet (not just paused)", () => {
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={NOT_STARTED_REASON}
        canResumeToUnlockPublicReply={false}
        onResumeWork={vi.fn()}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "Resume work" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText(NOT_STARTED_REASON)).toBeInTheDocument();
  });

  it("does not show the resume link when public replies are already allowed", () => {
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={null}
        canResumeToUnlockPublicReply
        onResumeWork={vi.fn()}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "Resume work" }),
    ).not.toBeInTheDocument();
  });

  it("disables the resume link and shows a pending label while resuming", () => {
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={PAUSED_REASON}
        canResumeToUnlockPublicReply
        onResumeWork={vi.fn()}
        isResumingWork
      />,
    );

    expect(screen.getByRole("button", { name: "Resuming…" })).toBeDisabled();
  });
});

/** A File-like drop payload — jsdom's DataTransfer doesn't populate `files`
 * from a plain object literal the way a real browser drag event would. */
function fileDrop(files: File[]): { dataTransfer: { types: string[]; files: File[] } } {
  return { dataTransfer: { types: ["Files"], files } };
}

function makeFile(name: string, sizeBytes: number, type = "text/plain"): File {
  return new File([new Uint8Array(sizeBytes)], name, { type });
}

describe("CsmCaseCommentInput — drag-and-drop attachments", () => {
  it("shows the drop overlay only while a file is being dragged over the composer", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    const composer = screen.getByTestId("csm-comment-composer");

    expect(screen.queryByText("Drop files to attach")).not.toBeInTheDocument();

    fireEvent.dragEnter(composer, fileDrop([]));
    expect(screen.getByText("Drop files to attach")).toBeInTheDocument();

    fireEvent.dragLeave(composer, fileDrop([]));
    expect(screen.queryByText("Drop files to attach")).not.toBeInTheDocument();
  });

  it("attaches a dropped file without opening the naming modal", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    const composer = screen.getByTestId("csm-comment-composer");

    fireEvent.drop(composer, fileDrop([makeFile("screenshot.png", 1024, "image/png")]));

    expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
    expect(screen.queryByText("Attach a file")).not.toBeInTheDocument();
  });

  it("attaches every file from a multi-file drop", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    const composer = screen.getByTestId("csm-comment-composer");

    fireEvent.drop(
      composer,
      fileDrop([makeFile("a.txt", 100), makeFile("b.txt", 200), makeFile("c.txt", 300)]),
    );

    expect(screen.getByTestId("attachment-count")).toHaveTextContent("3");
  });

  it("rejects a dropped file over the size limit but keeps the ones that fit", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    const composer = screen.getByTestId("csm-comment-composer");

    const tooBig = makeFile("huge.zip", 11 * 1024 * 1024);
    fireEvent.drop(composer, fileDrop([makeFile("ok.txt", 100), tooBig]));

    expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
    expect(screen.getByText(/huge\.zip.*too large/i)).toBeInTheDocument();
  });

  it("still prevents the browser's default file handling when the composer is disabled, without attaching anything", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} disabled />);
    const composer = screen.getByTestId("csm-comment-composer");

    // fireEvent returns false when preventDefault() was called on a
    // cancelable event — a real file drag must still be prevented even
    // though the composer won't act on it, or the browser falls through to
    // navigating the tab to open the dropped file.
    const notPrevented = fireEvent.drop(composer, fileDrop([makeFile("a.txt", 100)]));

    expect(notPrevented).toBe(false);
    expect(screen.getByTestId("attachment-count")).toHaveTextContent("0");
  });

  it("dedupes a file that's already attached", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    const composer = screen.getByTestId("csm-comment-composer");
    const file = makeFile("dup.txt", 100);

    fireEvent.drop(composer, fileDrop([file]));
    fireEvent.drop(composer, fileDrop([file]));

    expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
  });

  it("ignores a drag that isn't carrying files (e.g. dragging selected text)", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    const composer = screen.getByTestId("csm-comment-composer");

    fireEvent.dragEnter(composer, { dataTransfer: { types: ["text/plain"], files: [] } });
    expect(screen.queryByText("Drop files to attach")).not.toBeInTheDocument();
  });
});
