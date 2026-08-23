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

import type { SubscriptionType } from "@features/csm-projects/types/csmProjects";

/**
 * Cloud-support subscription types. For these, a case is filed against the
 * project's single primary-production deployment, so the case-creation form
 * hides the deployment picker and auto-selects it (mirrors the customer
 * portal's `shouldRestrictToPrimaryProductionDeployments`). Managed-cloud and
 * other subscriptions keep the normal deployment → product cascade.
 */
export const CLOUD_SUPPORT_SUBSCRIPTION_TYPES: readonly SubscriptionType[] = [
  "cloud_support",
  "cloud_evaluation_support",
];

/** True when the subscription type files cases directly against the primary
 *  production deployment (no deployment selection step). */
export function isCloudSupportSubscription(
  type: SubscriptionType | null | undefined,
): boolean {
  return !!type && CLOUD_SUPPORT_SUBSCRIPTION_TYPES.includes(type);
}

/**
 * Client-side mirror of the backing data source's project-type feature
 * matrix for service-request eligibility. The matrix is server-enforced —
 * this is UX only, to stop an engineer filling out a form the backend will
 * reject.
 */
export interface ServiceRequestEligibility {
  eligible: boolean;
  /** True when the ineligibility should stop the engineer proceeding (vs.
   *  just a heads-up). */
  blocking: boolean;
  message?: string;
}

/**
 * Subscription types the feature matrix always allows to raise a service
 * request.
 */
const SR_ELIGIBLE_SUBSCRIPTION_TYPES: readonly SubscriptionType[] = [
  "managed_cloud_subscription",
];

/**
 * Determine whether a project's subscription type can raise a service
 * request, per the backing data source's project-type feature matrix.
 * `cloud_support` gets a soft, non-blocking warning rather than a hard block:
 * that tier's real eligibility depends on the project's product category
 * (whether it includes a PDP), a signal this frontend has no access to today
 * — only the backend can give a definitive answer, so the UI can't safely
 * block here without risking a false negative.
 */
export function getServiceRequestEligibility(
  type: SubscriptionType | null | undefined,
): ServiceRequestEligibility {
  if (!type || SR_ELIGIBLE_SUBSCRIPTION_TYPES.includes(type)) {
    return { eligible: true, blocking: false };
  }
  if (type === "cloud_support") {
    return {
      eligible: true,
      blocking: false,
      message:
        "This project's service request eligibility depends on its product configuration — the backing data source will confirm on submit.",
    };
  }
  return {
    eligible: false,
    blocking: true,
    message:
      "This project's subscription type isn't eligible for service requests.",
  };
}
