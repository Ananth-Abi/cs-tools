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
 * The "optional" middle columns `CasesList` can render between Subject and
 * State — everything a caller can add, remove, or reorder via
 * `ColumnCustomizerButton`. Case ID, Subject, State, and Updated are never
 * optional: they carry the row's identity, its own sort control, or (Case ID)
 * the row's real anchor link, so removing them would break navigation/sort
 * rather than just decluttering.
 */
export type CaseOptionalColumnId = "product" | "type" | "severity" | "assignee";

export const CASE_OPTIONAL_COLUMNS: Record<
  CaseOptionalColumnId,
  { label: string; track: string }
> = {
  product: { label: "Product", track: "minmax(140px, 1fr)" },
  type: { label: "Type", track: "auto" },
  severity: { label: "Severity", track: "auto" },
  assignee: { label: "Assignee", track: "minmax(140px, 1fr)" },
};
