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

import { describe, expect, it } from "vitest";
import { ConfigError, resolveBackendBaseUrl } from "@config/apiConfig";

describe("resolveBackendBaseUrl", () => {
  it("returns the URL without trailing slashes", () => {
    expect(resolveBackendBaseUrl("https://api.example.com/kb/v1/")).toBe("https://api.example.com/kb/v1");
  });

  it.each([undefined, null, "", "   ", 42])("throws a named error for %s", (value) => {
    expect(() => resolveBackendBaseUrl(value)).toThrow(ConfigError);
    expect(() => resolveBackendBaseUrl(value)).toThrow(/KNOWLEDGE_PORTAL_BACKEND_BASE_URL/);
  });

  it.each(["<your-backend-base-url>", "api.example.com", "ftp://example.com", "javascript:alert(1)"])(
    "rejects %s",
    (value) => {
      expect(() => resolveBackendBaseUrl(value)).toThrow(ConfigError);
    },
  );
});
