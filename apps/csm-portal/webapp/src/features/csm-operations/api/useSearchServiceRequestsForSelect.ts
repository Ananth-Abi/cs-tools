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

import { keepPreviousData, useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCaseFieldFilter,
  BeCaseSearchPayload,
  BeCaseSearchResponse,
  BeCaseSearchView,
} from "@api/backend/types";

/** A single page of matches is plenty for a type-ahead picker. */
const SERVICE_REQUEST_SEARCH_LIMIT = 20;

const TYPE_FILTER: BeCaseFieldFilter = {
  field: "type",
  op: "in",
  values: ["service_request"],
};

/**
 * Type-ahead service-request search (`POST /cases/search`,
 * `filters.searchQuery`) for the change-request create form's "Originating
 * service request" picker. Matches the `(query, enabled, extra?) => {data,
 * isFetching, isError}` shape `AsyncEntitySelect` expects — same template as
 * `useSearchCasesForSelect`. Pinned to `types: ["service_request"]` since the
 * picker links a change request back to the service request it was raised
 * from, never a plain case.
 *
 * `projectId` (threaded through `AsyncEntitySelect`'s `searchExtra`) narrows
 * to service requests on the same project as the case the create form was
 * opened from — set only when the create form was reached via a service
 * request's own "Create change request" action, which is the only entry
 * point today that carries any project context (the create form has no
 * project field of its own to seed it otherwise). This is deliberately not a
 * hard filter: `/cases/search`'s field-filter DSL has no `accountId` field
 * (only `projectId` — see `BeCaseFieldFilterField`), and a project-scoped
 * search that comes back empty falls straight through to the same unscoped,
 * system-wide search this hook has always run, so an incomplete or
 * mismatched `projectId` can narrow the *default* suggestions but never
 * exclude a real match the unscoped search would have found.
 */
export function useSearchServiceRequestsForSelect(
  query: string,
  enabled: boolean,
  projectId?: string,
): UseQueryResult<BeCaseSearchView[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<BeCaseSearchView[], Error>({
    queryKey: [ApiQueryKeys.SERVICE_REQUESTS_SEARCH_FOR_SELECT, q, projectId ?? ""],
    queryFn: async (): Promise<BeCaseSearchView[]> => {
      const search = async (filters: BeCaseFieldFilter[]): Promise<BeCaseSearchView[]> => {
        const res = await api.post<BeCaseSearchPayload, BeCaseSearchResponse>(
          "/cases/search",
          {
            filters: { searchQuery: q, filters },
            pagination: { offset: 0, limit: SERVICE_REQUEST_SEARCH_LIMIT },
          },
        );
        return res.cases ?? [];
      };

      if (projectId) {
        const scoped = await search([
          TYPE_FILTER,
          { field: "projectId", op: "in", values: [projectId] },
        ]);
        if (scoped.length > 0) return scoped;
      }
      return search([TYPE_FILTER]);
    },
    enabled,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}
