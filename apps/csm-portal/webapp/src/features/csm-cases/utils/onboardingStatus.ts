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
 * The fixed ServiceNow choice list for `projectOnboardingStatus`. Shared
 * between the "Onboarding status" bar control (`CasesFilterBar.tsx`, which
 * offers exactly these 4 as options) and `translateCaseDashboardFilters`
 * (`widgetResourceConfig.ts`, which needs the full set to turn a dashboard
 * widget's `notIn` filter into this field's complement — see that file's
 * doc comment for why).
 */
export const ALL_ONBOARDING_STATUSES = [
  "In-Progress",
  "Not-Started",
  "Completed",
  "Not-Applicable",
] as const;

export const ONBOARDING_STATUS_LABEL: Record<string, string> = {
  "In-Progress": "In progress",
  "Not-Started": "Not started",
  Completed: "Completed",
  "Not-Applicable": "Not applicable",
};
