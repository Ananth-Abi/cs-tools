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
  Box,
  Container,
  Divider,
  Grid,
  List,
  ListItemButton,
  ListItemText,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { Link, useParams, useSearchParams } from "react-router";
import { isValidId } from "@api/backend/types";
import { isClientError, isNotFoundError } from "@api/backend/client";
import { useArticleSearch, useKnowledgeBases } from "@api/useKb";
import EmptyState from "@components/EmptyState";
import LoadingState from "@components/LoadingState";
import QueryErrorState from "@components/QueryErrorState";
import ArticleCard from "@features/kb/components/ArticleCard";
import ArticleList, { ArticleListItem } from "@features/kb/components/ArticleList";
import LoadMoreButton from "@features/kb/components/LoadMoreButton";
import SearchBox from "@features/kb/components/SearchBox";
import { useDocumentTitle } from "@hooks/useDocumentTitle";
import { browseUrl } from "@utils/browseUrl";
import { normalizeSearchQuery } from "@utils/searchQuery";
import { toSnippet } from "@utils/snippet";

export default function KBBrowsePage(): JSX.Element {
  const [params, setParams] = useSearchParams();

  const { id: rawKb } = useParams();
  const knowledgeBaseId = isValidId(rawKb) ? rawKb : "";
  const searchQuery = normalizeSearchQuery(params.get("q"));

  const { data: kbData } = useKnowledgeBases();
  const search = useArticleSearch({ knowledgeBaseId, searchQuery });

  const knowledgeBases = kbData?.knowledgeBases ?? [];
  const selectedKb = knowledgeBases.find((kb) => kb.id === knowledgeBaseId);

  const heading = searchQuery
    ? `Search results for "${searchQuery}"`
    : (selectedKb?.title ?? "All articles");
  useDocumentTitle(heading);

  const onSearch = (raw: string): void => {
    const next = new URLSearchParams(params);
    const q = normalizeSearchQuery(raw);
    if (q) next.set("q", q);
    else next.delete("q");
    setParams(next);
  };

  const { articles, total } = search;
  const showInitialError = search.isError && articles.length === 0;
  const unknownKnowledgeBase = !!knowledgeBaseId && isNotFoundError(search.error);

  // With a knowledge base filter the backend's total is exact. Without one the
  // backend filters each upstream page afterwards, so total only bounds the
  // real count from above: say so, or say nothing.
  let countLine = "";
  if (articles.length > 0 || !search.hasMore) {
    if (knowledgeBaseId) {
      countLine = `${total} article${total === 1 ? "" : "s"}`;
      if (articles.length > 0 && total > articles.length) countLine += `, showing ${articles.length}`;
    } else if (search.hasMore) {
      countLine = `Showing ${articles.length} of up to ${total} articles`;
    } else {
      countLine = `${articles.length} article${articles.length === 1 ? "" : "s"}`;
    }
  }

  return (
    <Container maxWidth="lg" sx={{ py: 5 }}>
      <Typography variant="h4" component="h1" sx={{ fontWeight: 700, mb: 3 }}>
        {heading}
      </Typography>

      <Box sx={{ mb: 4 }}>
        {/* Remount when the URL query changes so the input follows back/forward. */}
        <SearchBox
          key={searchQuery}
          label="Search articles"
          placeholder="Search articles..."
          initialValue={searchQuery}
          onSearch={onSearch}
        />
      </Box>

      <Grid container spacing={4}>
        <Grid size={{ xs: 12, md: 3 }}>
          <Box component="nav" aria-label="Knowledge bases">
            <Typography variant="subtitle2" component="h2" sx={{ fontWeight: 600, mb: 1 }}>
              Knowledge bases
            </Typography>
            <Divider sx={{ mb: 1 }} />
            <List dense disablePadding>
              <ListItemButton
                component={Link}
                to={browseUrl({ q: searchQuery })}
                selected={!knowledgeBaseId}
                aria-current={!knowledgeBaseId ? "page" : undefined}
                sx={{ borderRadius: 1 }}
              >
                <ListItemText primary="All" />
              </ListItemButton>
              {knowledgeBases.map((kb) => (
                <ListItemButton
                  key={kb.id}
                  component={Link}
                  to={browseUrl({ knowledgeBaseId: kb.id, q: searchQuery })}
                  selected={kb.id === knowledgeBaseId}
                  aria-current={kb.id === knowledgeBaseId ? "page" : undefined}
                  sx={{ borderRadius: 1 }}
                >
                  <ListItemText primary={kb.title} />
                </ListItemButton>
              ))}
            </List>
          </Box>
        </Grid>

        <Grid size={{ xs: 12, md: 9 }}>
          {search.isLoading && <LoadingState label="Loading articles" />}

          {search.isSkippingEmpty && articles.length === 0 && !search.isLoading && (
            <LoadingState label="Loading articles" />
          )}

          {showInitialError && (
            <QueryErrorState
              error={search.error}
              message={
                unknownKnowledgeBase
                  ? "This knowledge base does not exist or is no longer available."
                  : "Articles could not be loaded."
              }
              onRetry={isClientError(search.error) ? undefined : search.refetch}
            />
          )}

          {!search.isLoading && !showInitialError && !(search.isSkippingEmpty && articles.length === 0) && (
            <>
              {countLine && (
                <Typography
                  variant="body2"
                  color="text.secondary"
                  role="status"
                  aria-live="polite"
                  sx={{ mb: 2 }}
                >
                  {countLine}
                </Typography>
              )}

              {articles.length === 0 ? (
                search.hasMore ? (
                  <EmptyState
                    title="No articles found yet"
                    message="The articles checked so far did not match. Load more to keep looking."
                  />
                ) : (
                  <EmptyState
                    title="No articles matched"
                    message="Try a different search or knowledge base."
                  />
                )
              ) : (
                <ArticleList label="Articles" busy={search.isRefreshing}>
                  {articles.map((article) => (
                    <ArticleListItem key={article.id}>
                      <ArticleCard
                        id={article.id}
                        title={article.title}
                        snippet={toSnippet(article.body)}
                        publishedOn={article.publishedOn}
                      />
                    </ArticleListItem>
                  ))}
                </ArticleList>
              )}

              {search.isLoadMoreError && (
                <QueryErrorState
                  error={search.error}
                  message="More articles could not be loaded."
                />
              )}

              {search.hasMore && (
                <LoadMoreButton onClick={search.loadMore} loading={search.isLoadingMore} />
              )}

              {search.isCapped && (
                <Typography variant="body2" color="text.secondary" role="status" sx={{ mt: 3, textAlign: "center" }}>
                  Only the first {articles.length} results can be shown. Refine your search or pick a
                  knowledge base to narrow them down.
                </Typography>
              )}
            </>
          )}
        </Grid>
      </Grid>
    </Container>
  );
}
