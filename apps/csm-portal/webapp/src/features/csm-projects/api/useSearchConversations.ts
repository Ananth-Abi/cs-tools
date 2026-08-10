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
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeConversationView,
  BeSearchConversationsPayload,
  BeSearchConversationsResponse,
} from "@api/backend/types";

/** Zero-indexed page + page size, mirroring MUI `TablePagination`. */
export interface ConversationPagination {
  page: number;
  rowsPerPage: number;
}

export interface ConversationSearchResult {
  conversations: BeConversationView[];
  total: number;
}

/**
 * A project's chat sessions, via `POST /conversations/search`, sorted most
 * recently active first. `rowsPerPage` is capped at {@link BE_MAX_PAGE_LIMIT}
 * (the entity service's own documented max for this endpoint). Disabled until
 * a project id is provided.
 */
export function useSearchConversations(
  projectId: string | undefined,
  pagination: ConversationPagination,
): UseQueryResult<ConversationSearchResult, Error> {
  const api = useBackendApi();
  const limit = Math.min(pagination.rowsPerPage, BE_MAX_PAGE_LIMIT);
  const offset = pagination.page * limit;

  return useQuery<ConversationSearchResult, Error>({
    queryKey: [ApiQueryKeys.CONVERSATIONS_SEARCH, projectId ?? "", pagination.page, limit],
    queryFn: async (): Promise<ConversationSearchResult> => {
      const res = await api.post<
        BeSearchConversationsPayload,
        BeSearchConversationsResponse
      >("/conversations/search", {
        filters: { projectIds: [projectId ?? ""] },
        sortBy: { field: "updatedOn", order: "desc" },
        pagination: { limit, offset },
      });
      return { conversations: res.conversations ?? [], total: res.total };
    },
    enabled: !!projectId,
    staleTime: 30_000,
  });
}
