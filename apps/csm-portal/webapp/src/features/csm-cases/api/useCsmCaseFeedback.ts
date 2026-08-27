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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCaseFeedback,
  BeCaseFeedbackSearchFilters,
  BeCaseFeedbackSearchResponse,
} from "@api/backend/types";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import type { CaseFeedbackEntry } from "@features/csm-cases/types/csmCases";

function feedbackEntryFromBe(f: BeCaseFeedback): CaseFeedbackEntry {
  return {
    id: f.instanceId,
    rating: f.rating,
    ratingLabel: f.ratingLabel,
    comment: f.comment,
    submittedAt: f.submittedAt,
  };
}

/**
 * Loads any Case Feedback survey submissions for a single case, for the case
 * detail page's activity feed. Case Feedback is a CSAT survey submitted by
 * the customer, typically only once a case is closed — an open case will
 * almost always resolve to an empty list, which is expected, not an error.
 *
 * Reuses `WIDGET_RESOURCE_CONFIG.case_feedback`'s endpoint rather than
 * hardcoding the path, so this stays in sync with that config's own source
 * of truth for the endpoint name.
 */
export function useGetCsmCaseFeedback(
  caseId: string | undefined,
): UseQueryResult<CaseFeedbackEntry[], Error> {
  const api = useBackendApi();

  return useQuery<CaseFeedbackEntry[], Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_FEEDBACK, caseId ?? ""],
    queryFn: async (): Promise<CaseFeedbackEntry[]> => {
      if (!caseId) return [];

      const payload = {
        filters: { caseId } satisfies BeCaseFeedbackSearchFilters,
        page: 1,
        pageSize: 20,
      };
      const response = await api.post<
        typeof payload,
        BeCaseFeedbackSearchResponse
      >(WIDGET_RESOURCE_CONFIG.case_feedback.searchEndpoint, payload);
      return (response.results ?? []).map(feedbackEntryFromBe);
    },
    enabled: !!caseId,
    staleTime: 10_000,
  });
}
