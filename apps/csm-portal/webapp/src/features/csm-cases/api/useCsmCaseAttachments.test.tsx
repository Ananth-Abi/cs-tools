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

import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

// `vi.mock` factories are hoisted above top-level `const`s, so anything a
// factory below closes over must itself be created via `vi.hoisted`.
const { postMock, getBlobMock, uploadFileViaTusMock, sftpgoFlag } = vi.hoisted(
  () => ({
    postMock: vi.fn(),
    getBlobMock: vi.fn(),
    uploadFileViaTusMock: vi.fn(),
    sftpgoFlag: { enabled: false },
  }),
);

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useSearchTags.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock, getBlob: getBlobMock }),
}));

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { sftpgoAttachmentStorageEnabled: sftpgoFlag.enabled },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/csm-cases/api/attachmentStorageTus", () => ({
  uploadFileViaTus: uploadFileViaTusMock,
}));

vi.mock("@utils/saveBlob", () => ({ saveBlob: vi.fn() }));

import {
  usePostCsmCaseAttachment,
  useDownloadCsmCaseAttachment,
} from "@features/csm-cases/api/useCsmCaseAttachments";
import { saveBlob } from "@utils/saveBlob";
import type { CaseAttachment } from "@features/csm-cases/types/csmCases";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

const FILE = new File(["hello world"], "hello.txt", { type: "text/plain" });

describe("usePostCsmCaseAttachment", () => {
  beforeEach(() => {
    postMock.mockReset();
    getBlobMock.mockReset();
    uploadFileViaTusMock.mockReset();
    sftpgoFlag.enabled = false;
  });

  it("flag off: sends a single POST /attachments with a base64 file payload, unchanged", async () => {
    postMock.mockResolvedValue({});
    const { result } = renderHook(() => usePostCsmCaseAttachment(), {
      wrapper,
    });

    await act(() =>
      result.current.mutateAsync({
        caseId: "case-1",
        file: FILE,
        uploadedBy: "Jane Doe",
      }),
    );

    expect(uploadFileViaTusMock).not.toHaveBeenCalled();
    expect(postMock).toHaveBeenCalledTimes(1);
    const [path, payload] = postMock.mock.calls[0];
    expect(path).toBe("/attachments");
    expect(payload.storageKey).toBeUndefined();
    expect(payload.sizeBytes).toBeUndefined();
    expect(typeof payload.file).toBe("string");
    expect(payload.file.startsWith("data:")).toBe(true);
    expect(result.current.uploadProgress).toBeNull();
  });

  it("flag on: mints an upload token, uploads via TUS, then creates metadata with storageKey/sizeBytes and no file", async () => {
    sftpgoFlag.enabled = true;
    postMock.mockImplementation((path: string) => {
      if (path.endsWith("/attachments/upload-token")) {
        return Promise.resolve({
          sftpgoAccessToken: "tok-1",
          expiresAt: "2026-01-01T00:00:00Z",
          sftpgoBaseUrl: "https://sftpgo.example.com",
          storageKey: "/attachments/cases/case-1/att-1",
        });
      }
      return Promise.resolve({});
    });
    uploadFileViaTusMock.mockResolvedValue(undefined);

    const { result } = renderHook(() => usePostCsmCaseAttachment(), {
      wrapper,
    });

    await act(() =>
      result.current.mutateAsync({
        caseId: "case-1",
        file: FILE,
        uploadedBy: "Jane Doe",
      }),
    );

    // Order: mint token, then TUS upload, then metadata create.
    expect(postMock).toHaveBeenCalledTimes(2);
    expect(postMock.mock.calls[0][0]).toBe(
      "/cases/case-1/attachments/upload-token",
    );
    expect(uploadFileViaTusMock).toHaveBeenCalledTimes(1);
    expect(uploadFileViaTusMock.mock.calls[0][0]).toMatchObject({
      sftpgoBaseUrl: "https://sftpgo.example.com",
      sftpgoAccessToken: "tok-1",
      storageKey: "/attachments/cases/case-1/att-1",
      file: FILE,
    });

    const [metaPath, metaPayload] = postMock.mock.calls[1];
    expect(metaPath).toBe("/attachments");
    expect(metaPayload.storageKey).toBe("/attachments/cases/case-1/att-1");
    expect(metaPayload.sizeBytes).toBe(FILE.size);
    expect(metaPayload.file).toBeUndefined();

    // The TUS call must have happened before the metadata create.
    const tusCallOrder = uploadFileViaTusMock.mock.invocationCallOrder[0];
    const metaCallOrder = postMock.mock.invocationCallOrder[1];
    expect(tusCallOrder).toBeLessThan(metaCallOrder);

    expect(result.current.uploadProgress).toBeNull();
  });

  it("flag on: exposes upload progress from the TUS callback while in flight", async () => {
    sftpgoFlag.enabled = true;
    postMock.mockImplementation((path: string) => {
      if (path.endsWith("/attachments/upload-token")) {
        return Promise.resolve({
          sftpgoAccessToken: "tok-1",
          expiresAt: "2026-01-01T00:00:00Z",
          sftpgoBaseUrl: "https://sftpgo.example.com",
          storageKey: "/attachments/cases/case-1/att-1",
        });
      }
      return Promise.resolve({});
    });

    // A deferred promise the test resolves explicitly, so the upload is
    // guaranteed to still be "in flight" while the progress assertion below
    // runs — a `setTimeout(0)` here would race the real TUS call's resolution
    // against `waitFor`'s own polling and could resolve before the test gets
    // a chance to observe the in-flight percentage.
    let resolveUpload: (() => void) | undefined;
    let capturedOnProgress: ((percent: number) => void) | undefined;
    uploadFileViaTusMock.mockImplementation(({ onProgress }) => {
      capturedOnProgress = onProgress;
      return new Promise<void>((resolve) => {
        resolveUpload = resolve;
      });
    });

    const { result } = renderHook(() => usePostCsmCaseAttachment(), {
      wrapper,
    });

    let promise!: Promise<unknown>;
    act(() => {
      promise = result.current.mutateAsync({
        caseId: "case-1",
        file: FILE,
        uploadedBy: "Jane Doe",
      });
    });

    await waitFor(() => expect(capturedOnProgress).toBeDefined());
    act(() => capturedOnProgress!(42));
    await waitFor(() => expect(result.current.uploadProgress).toBe(42));

    await act(async () => {
      resolveUpload!();
      await promise;
    });
    await waitFor(() => expect(result.current.uploadProgress).toBeNull());
  });
});

describe("useDownloadCsmCaseAttachment", () => {
  const ATTACHMENT: CaseAttachment = {
    id: "att-1",
    filename: "hello.txt",
    size: 11,
    contentType: "text/plain",
    uploadedBy: "Jane Doe",
    uploadedAt: "2026-01-01T00:00:00Z",
  };

  beforeEach(() => {
    postMock.mockReset();
    getBlobMock.mockReset();
    sftpgoFlag.enabled = false;
    vi.mocked(saveBlob).mockReset();
  });

  it("flag off: fetches the content blob and saves it, unchanged", async () => {
    getBlobMock.mockResolvedValue(new Blob(["hello"], { type: "text/plain" }));
    const { result } = renderHook(() => useDownloadCsmCaseAttachment(), {
      wrapper,
    });

    await result.current(ATTACHMENT);

    expect(postMock).not.toHaveBeenCalled();
    expect(getBlobMock).toHaveBeenCalledWith("/attachments/att-1/content");
    expect(saveBlob).toHaveBeenCalledTimes(1);
  });

  it("flag on: creates exactly one share for the clicked attachment and never fetches the content blob", async () => {
    sftpgoFlag.enabled = true;
    postMock.mockResolvedValue({
      shareUrl: "https://sftpgo.example.com/web/client/pubshares/abc",
    });
    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => {});

    const { result } = renderHook(() => useDownloadCsmCaseAttachment(), {
      wrapper,
    });

    await result.current(ATTACHMENT);

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith("/attachments/att-1/share", {});
    expect(getBlobMock).not.toHaveBeenCalled();
    expect(saveBlob).not.toHaveBeenCalled();
    expect(clickSpy).toHaveBeenCalledTimes(1);

    clickSpy.mockRestore();
  });
});
