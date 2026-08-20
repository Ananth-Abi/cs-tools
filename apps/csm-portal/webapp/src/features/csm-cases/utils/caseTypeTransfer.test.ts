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
import {
  ALWAYS_RETAINED_FIELDS,
  computeTransferPreview,
  SUPPORTED_TRANSFER_TARGETS,
  TRANSFERABLE_CASE_TYPES,
} from "@features/csm-cases/utils/caseTypeTransfer";

describe("computeTransferPreview", () => {
  it("loses severity and issue type, gains a required engagement type, moving case -> engagement", () => {
    const preview = computeTransferPreview("case", "engagement", true);
    expect(preview.lostFields.map((f) => f.key)).toEqual(["severity", "issueType"]);
    expect(preview.neededFields.map((f) => f.key)).toEqual(["engagementType"]);
    expect(preview.attachmentNeeded).toBe(false);
    expect(preview.catalogNeeded).toBe(false);
  });

  it("loses engagement type and offers severity/issue type to fill in, moving engagement -> case", () => {
    const preview = computeTransferPreview("engagement", "case", true);
    expect(preview.lostFields.map((f) => f.key)).toEqual(["engagementType"]);
    // Case's own specific fields are offered here, but — see the
    // entity-service validator — only engagementType is actually required
    // to complete a transfer; severity/issueType are optional extras the
    // dialog collects for data completeness, not a submit blocker.
    expect(preview.neededFields.map((f) => f.key)).toEqual(["severity", "issueType"]);
  });

  it("flags a missing attachment as needed moving into security_report_analysis", () => {
    const preview = computeTransferPreview("case", "security_report_analysis", false);
    expect(preview.attachmentNeeded).toBe(true);
  });

  it("does not flag an attachment as needed when the case already has one", () => {
    const preview = computeTransferPreview("case", "security_report_analysis", true);
    expect(preview.attachmentNeeded).toBe(false);
  });

  it("loses nothing moving out of security_report_analysis, which has no specific fields", () => {
    const preview = computeTransferPreview("security_report_analysis", "engagement", true);
    expect(preview.lostFields).toEqual([]);
  });

  it("flags the catalog picker as needed moving into service_request", () => {
    const preview = computeTransferPreview("case", "service_request", true);
    expect(preview.catalogNeeded).toBe(true);
    expect(preview.neededFields).toEqual([]);
  });

  it("does not flag the catalog picker moving out of service_request", () => {
    const preview = computeTransferPreview("service_request", "case", true);
    expect(preview.catalogNeeded).toBe(false);
  });
});

describe("ALWAYS_RETAINED_FIELDS", () => {
  it("is a non-empty, fixed list independent of any pair", () => {
    expect(ALWAYS_RETAINED_FIELDS.length).toBeGreaterThan(0);
    expect(ALWAYS_RETAINED_FIELDS).toContain("Subject");
    expect(ALWAYS_RETAINED_FIELDS).toContain("Existing attachments");
  });
});

describe("TRANSFERABLE_CASE_TYPES / SUPPORTED_TRANSFER_TARGETS", () => {
  it("excludes announcement from the transferable set", () => {
    expect(TRANSFERABLE_CASE_TYPES).not.toContain("announcement");
    expect(TRANSFERABLE_CASE_TYPES).toHaveLength(4);
  });

  it("only supports case and engagement as real submit targets today", () => {
    expect(SUPPORTED_TRANSFER_TARGETS).toEqual(["case", "engagement"]);
  });
});
