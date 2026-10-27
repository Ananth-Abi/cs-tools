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

import {
  keepPreviousData,
  useInfiniteQuery,
  useQuery,
} from "@tanstack/react-query";
import {
  getArticle,
  getRelatedArticles,
  listKnowledgeBases,
  searchArticles,
} from "@api/backend/client";
import { isValidId } from "@api/backend/types";
import type { KBArticle } from "@api/backend/types";
import { getNextOffset, PAGE_SIZE } from "@api/pagination";

const KB_KEY = "kb";
const FIVE_MINUTES = 5 * 60 * 1000;

export const useKnowledgeBases = () =>
  useQuery({
    queryKey: [KB_KEY, "knowledge-bases"],
    queryFn: ({ signal }) => listKnowledgeBases(signal),
    staleTime: FIVE_MINUTES,
  });

export const useRecentArticles = (limit: number) =>
  useQuery({
    queryKey: [KB_KEY, "recent", limit],
    queryFn: ({ signal }) => searchArticles({ sortBy: "recent", limit }, signal),
  });

export interface ArticleSearchFilters {
  knowledgeBaseId?: string;
  searchQuery?: string;
}

/**
 * Paged article search. Pages accumulate (`articles` is every page loaded so
 * far) and `loadMore` fetches the next one. While a new filter is loading the
 * previous results stay on screen instead of collapsing to a spinner.
 */
export function useArticleSearch({ knowledgeBaseId, searchQuery }: ArticleSearchFilters) {
  const query = useInfiniteQuery({
    queryKey: [KB_KEY, "search", knowledgeBaseId ?? "", searchQuery ?? ""],
    initialPageParam: 0,
    queryFn: ({ pageParam, signal }) =>
      searchArticles(
        {
          ...(knowledgeBaseId ? { knowledgeBaseId } : {}),
          ...(searchQuery ? { searchQuery } : {}),
          limit: PAGE_SIZE,
          offset: pageParam,
        },
        signal,
      ),
    getNextPageParam: getNextOffset,
    placeholderData: keepPreviousData,
  });

  const pages = query.data?.pages ?? [];
  const last = pages[pages.length - 1];

  return {
    articles: pages.flatMap((page) => page.articles),
    total: last?.total ?? 0,
    hasMore: query.hasNextPage,
    loadMore: () => void query.fetchNextPage(),
    refetch: () => void query.refetch(),
    isLoading: query.isLoading,
    /** True while a filter change is loading and old results are still shown. */
    isRefreshing: query.isPlaceholderData && query.isFetching,
    isLoadingMore: query.isFetchingNextPage,
    isError: query.isError,
    /** The failed request was a "load more", so earlier pages are intact. */
    isLoadMoreError: query.isFetchNextPageError,
    error: query.error,
  };
}

export const useArticle = (id: string | undefined) =>
  useQuery<KBArticle>({
    queryKey: [KB_KEY, "article", id ?? ""],
    queryFn: ({ signal }) => getArticle(id as string, signal),
    enabled: isValidId(id),
  });

export const useRelatedArticles = (id: string | undefined) =>
  useQuery({
    queryKey: [KB_KEY, "related", id ?? ""],
    queryFn: ({ signal }) => getRelatedArticles(id as string, signal),
    enabled: isValidId(id),
    staleTime: FIVE_MINUTES,
  });
