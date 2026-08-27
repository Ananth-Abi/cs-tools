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

/**
 * Uploads a file directly from the browser to SFTPGo's chunked/TUS upload
 * endpoint, bypassing this app's own backend entirely (the backend only
 * mints the short-lived credential — see `usePostCsmCaseAttachment`).
 *
 * Deliberately NOT sent through `useBackendApi`/`useAuthApiClient`: that
 * client refuses to attach a bearer token to any origin other than this
 * app's own backend (see `useAuthApiClient.ts`), and the SFTPGo access token
 * minted for this upload is a different credential entirely — it must never
 * be confused with, or sent alongside, this app's own auth tokens.
 *
 * Follows the TUS resumable-upload protocol (POST to open an upload, then
 * PATCH to send the bytes): this is the wire shape SFTPGo's
 * `chunked-upload` endpoint was specified against for this change, but has
 * not been independently re-verified against a live SFTPGo instance — flagged
 * here the same way the backend's own `internal/sftpgo/client.go` flags its
 * own unverified SFTPGo API-shape assumptions (e.g. the share-scope and
 * share-id-header assumptions there).
 */

/** Base64-encodes a UTF-8 string for a TUS `Upload-Metadata` entry. */
function toBase64(value: string): string {
  // btoa operates on a byte string; encode to UTF-8 bytes first so a
  // non-ASCII file name doesn't throw.
  const bytes = new TextEncoder().encode(value);
  let binary = "";
  bytes.forEach((b) => {
    binary += String.fromCharCode(b);
  });
  return btoa(binary);
}

export interface UploadFileViaTusInput {
  sftpgoBaseUrl: string;
  sftpgoAccessToken: string;
  /** The exact SFTPGo path minted by `POST /cases/{id}/attachments/upload-token`. */
  storageKey: string;
  file: File;
  /** Called with 0-100 as the upload progresses. */
  onProgress?: (percent: number) => void;
  signal?: AbortSignal;
}

const TUS_RESUMABLE_VERSION = "1.0.0";

/**
 * Opens a TUS upload session (`POST`) for `storageKey`, then uploads the
 * file's bytes in a single `PATCH` at offset 0, reporting progress via
 * `onProgress`. Uses `XMLHttpRequest` for the `PATCH` rather than `fetch`:
 * `fetch` has no cross-browser-reliable upload progress event, while
 * `XMLHttpRequest.upload.onprogress` does.
 */
export async function uploadFileViaTus({
  sftpgoBaseUrl,
  sftpgoAccessToken,
  storageKey,
  file,
  onProgress,
  signal,
}: UploadFileViaTusInput): Promise<void> {
  const createEndpoint = `${sftpgoBaseUrl.replace(/\/+$/, "")}/api/v2/user/files/chunked-upload`;
  const uploadMetadata = [
    `path ${toBase64(storageKey)}`,
    `filename ${toBase64(file.name)}`,
  ].join(",");

  const createResponse = await fetch(createEndpoint, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${sftpgoAccessToken}`,
      "Tus-Resumable": TUS_RESUMABLE_VERSION,
      "Upload-Length": String(file.size),
      "Upload-Metadata": uploadMetadata,
    },
    signal,
  });
  if (!createResponse.ok) {
    throw new Error(
      `Failed to open the upload session (status ${createResponse.status}).`,
    );
  }

  // The TUS spec returns the upload's URL via `Location`, which may be
  // relative to the create endpoint's origin.
  const location = createResponse.headers.get("Location");
  const uploadUrl = location
    ? new URL(location, createEndpoint).toString()
    : createEndpoint;

  await patchWithProgress({
    url: uploadUrl,
    accessToken: sftpgoAccessToken,
    file,
    onProgress,
    signal,
  });
}

function patchWithProgress({
  url,
  accessToken,
  file,
  onProgress,
  signal,
}: {
  url: string;
  accessToken: string;
  file: File;
  onProgress?: (percent: number) => void;
  signal?: AbortSignal;
}): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PATCH", url, true);
    xhr.setRequestHeader("Authorization", `Bearer ${accessToken}`);
    xhr.setRequestHeader("Tus-Resumable", TUS_RESUMABLE_VERSION);
    xhr.setRequestHeader("Upload-Offset", "0");
    xhr.setRequestHeader("Content-Type", "application/offset+octet-stream");

    const onAbort = (): void => xhr.abort();
    if (signal) {
      if (signal.aborted) {
        reject(new DOMException("Aborted", "AbortError"));
        return;
      }
      signal.addEventListener("abort", onAbort);
    }

    xhr.upload.onprogress = (event) => {
      if (!onProgress || !event.lengthComputable) return;
      onProgress(Math.round((event.loaded / event.total) * 100));
    };

    xhr.onload = () => {
      signal?.removeEventListener("abort", onAbort);
      if (xhr.status >= 200 && xhr.status < 300) {
        onProgress?.(100);
        resolve();
      } else {
        reject(
          new Error(`Failed to upload the file (status ${xhr.status}).`),
        );
      }
    };
    xhr.onerror = () => {
      signal?.removeEventListener("abort", onAbort);
      reject(new Error("Failed to upload the file (network error)."));
    };
    xhr.onabort = () => {
      signal?.removeEventListener("abort", onAbort);
      reject(new DOMException("Aborted", "AbortError"));
    };

    xhr.send(file);
  });
}
