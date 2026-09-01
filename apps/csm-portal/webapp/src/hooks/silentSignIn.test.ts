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

import { describe, expect, it, vi } from "vitest";
import { trySilentSignInOnce } from "@hooks/silentSignIn";

describe("trySilentSignInOnce", () => {
  it("shares one in-flight attempt across two independent callers, as if from AuthGuard and useAuthApiClient at once", async () => {
    let resolveSignIn: (value: boolean) => void;
    const signInSilently = vi.fn(
      () =>
        new Promise<boolean>((resolve) => {
          resolveSignIn = resolve;
        }),
    );

    // Two independent callers (mirroring AuthGuard's route-mount check and
    // useAuthApiClient's fetch-401 recovery both noticing the same expired
    // token around the same moment) each start their own attempt.
    const first = trySilentSignInOnce(signInSilently);
    const second = trySilentSignInOnce(signInSilently);

    expect(signInSilently).toHaveBeenCalledTimes(1);

    resolveSignIn!(true);
    await expect(first).resolves.toBe(true);
    await expect(second).resolves.toBe(true);
  });

  it("starts a fresh attempt once the previous one has settled", async () => {
    const signInSilently = vi.fn().mockResolvedValueOnce(false).mockResolvedValueOnce(true);

    await expect(trySilentSignInOnce(signInSilently)).resolves.toBe(false);
    await expect(trySilentSignInOnce(signInSilently)).resolves.toBe(true);

    expect(signInSilently).toHaveBeenCalledTimes(2);
  });

  it("resolves false and reports a sanitized message when the attempt rejects", async () => {
    const signInSilently = vi.fn().mockRejectedValue(new Error("iframe blocked"));
    const onError = vi.fn();

    await expect(trySilentSignInOnce(signInSilently, onError)).resolves.toBe(false);

    expect(onError).toHaveBeenCalledWith("iframe blocked");
  });
});
