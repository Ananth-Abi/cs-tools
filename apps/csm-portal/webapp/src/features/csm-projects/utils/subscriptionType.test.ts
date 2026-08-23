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
import { getServiceRequestEligibility } from "./subscriptionType";
import type { SubscriptionType } from "@features/csm-projects/types/csmProjects";

describe("getServiceRequestEligibility", () => {
  it("is eligible, non-blocking, with no message for managed_cloud_subscription", () => {
    expect(getServiceRequestEligibility("managed_cloud_subscription")).toEqual({
      eligible: true,
      blocking: false,
    });
  });

  it("is eligible, non-blocking, with no message when the subscription type is unknown", () => {
    expect(getServiceRequestEligibility(undefined)).toEqual({
      eligible: true,
      blocking: false,
    });
    expect(getServiceRequestEligibility(null)).toEqual({
      eligible: true,
      blocking: false,
    });
  });

  it("gives cloud_support a soft, non-blocking warning (PDP-dependent, not knowable client-side)", () => {
    const result = getServiceRequestEligibility("cloud_support");
    expect(result.eligible).toBe(true);
    expect(result.blocking).toBe(false);
    expect(result.message).toMatch(/product configuration/i);
  });

  const HARD_BLOCKED: SubscriptionType[] = [
    "cloud_evaluation_support",
    "evaluation_subscription",
    "subscription",
    "development_support",
    "professional_services",
    "internal",
    "platformer_subscription",
  ];

  it.each(HARD_BLOCKED)("hard-blocks %s", (type) => {
    const result = getServiceRequestEligibility(type);
    expect(result.eligible).toBe(false);
    expect(result.blocking).toBe(true);
    expect(result.message).toBeTruthy();
  });
});
