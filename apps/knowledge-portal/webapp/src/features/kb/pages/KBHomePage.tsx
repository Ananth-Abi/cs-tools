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
  Card,
  CardActionArea,
  Container,
  Grid,
  Link as OxygenLink,
  Skeleton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowRight, BookOpen } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link, useNavigate } from "react-router";
import { useKnowledgeBases, useRecentArticles } from "@api/useKb";
import EmptyState from "@components/EmptyState";
import LoadingState from "@components/LoadingState";
import QueryErrorState from "@components/QueryErrorState";
import ArticleCard from "@features/kb/components/ArticleCard";
import ArticleList, { ArticleListItem } from "@features/kb/components/ArticleList";
import LoadMoreButton from "@features/kb/components/LoadMoreButton";
import SearchBox from "@features/kb/components/SearchBox";
import { accessibleLinkSx } from "@config/a11yThemeOverrides";
import { useDocumentTitle } from "@hooks/useDocumentTitle";
import { browseUrl } from "@utils/browseUrl";
import { normalizeSearchQuery } from "@utils/searchQuery";
import { toSnippet } from "@utils/snippet";

// Product cards shown on the landing page; the rest are reachable from /knowledge-bases.
const FEATURED_KB_LIMIT = 6;
const RECENT_LIMIT = 5;
// Fetch a bigger page than is shown: articles in inactive knowledge bases are
// dropped by the backend after paging, so a page of 5 can come back empty.
const RECENT_FETCH_LIMIT = 20;

export default function KBHomePage(): JSX.Element {
  useDocumentTitle();
  const navigate = useNavigate();
  const kbs = useKnowledgeBases();
  const recent = useRecentArticles(RECENT_FETCH_LIMIT);

  const allKnowledgeBases = kbs.data?.knowledgeBases ?? [];
  const featured = allKnowledgeBases.slice(0, FEATURED_KB_LIMIT);
  const hasMore = allKnowledgeBases.length > FEATURED_KB_LIMIT;
  const recentArticles = recent.articles.slice(0, RECENT_LIMIT);
  const recentLoading = recent.isLoading || (recent.isSkippingEmpty && recentArticles.length === 0);
  const recentFailed = recent.isError && recentArticles.length === 0;
  const recentEmpty = !recentLoading && !recentFailed && recentArticles.length === 0;

  return (
    <>
      <Box sx={{ borderBottom: 1, borderColor: "divider", bgcolor: "background.paper" }}>
        <Container maxWidth="md" sx={{ py: { xs: 6, md: 9 } }}>
          <Stack spacing={2.5} alignItems="center" textAlign="center">
            <Typography
              variant="h2"
              component="h1"
              sx={{
                fontWeight: 800,
                lineHeight: 1.1,
                fontSize: { xs: "2rem", sm: "2.75rem", md: "3.25rem" },
                maxWidth: 800,
              }}
            >
              WSO2 Knowledge Hub
            </Typography>
            <Typography variant="h6" component="p" color="text.secondary" sx={{ fontWeight: 400, maxWidth: 660 }}>
              Search all knowledge bases or browse a specific one. Find troubleshooting guides,
              how-to guides, FAQs (Frequently Asked Questions) and more.
            </Typography>
            <Box sx={{ width: "100%", maxWidth: 680, mt: 2 }}>
              <SearchBox
                large
                label="Search all articles"
                placeholder="Search for a product, error, or how-to..."
                onSearch={(q) => navigate(browseUrl({ q: normalizeSearchQuery(q) }))}
              />
            </Box>
          </Stack>
        </Container>
      </Box>

      <Container maxWidth="lg" sx={{ py: { xs: 5, md: 7 } }}>
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          sx={{ mb: 2.5 }}
        >
          <Typography variant="h5" component="h2" sx={{ fontWeight: 700 }}>
            Browse by product
          </Typography>
          {hasMore && (
            <OxygenLink
              component={Link}
              to={browseUrl()}
              underline="hover"
              sx={[
                { display: "inline-flex", alignItems: "center", gap: 0.5, fontWeight: 600 },
                accessibleLinkSx,
              ]}
            >
              View all knowledge bases
              <ArrowRight size={16} aria-hidden />
            </OxygenLink>
          )}
        </Stack>

        {kbs.isLoading && <LoadingState label="Loading knowledge bases" py={6} />}
        {kbs.isError && (
          <QueryErrorState
            error={kbs.error}
            message="The knowledge bases could not be loaded."
            onRetry={() => void kbs.refetch()}
          />
        )}
        {kbs.isSuccess && featured.length === 0 && (
          <EmptyState title="No knowledge bases yet" message="Check back soon." />
        )}

        {featured.length > 0 && (
          <Grid container spacing={2.5} component="ul" sx={{ mb: 7, p: 0, listStyle: "none" }}>
            {featured.map((kb) => (
              <Grid key={kb.id} component="li" size={{ xs: 12, sm: 6, md: 4 }}>
                <Card
                  variant="outlined"
                  sx={{
                    height: "100%",
                    transition: "border-color 120ms, transform 120ms",
                    "&:hover": { borderColor: "primary.main", transform: "translateY(-2px)" },
                  }}
                >
                  <CardActionArea
                    component={Link}
                    to={browseUrl({ knowledgeBaseId: kb.id })}
                    sx={{ p: 3, height: "100%", alignItems: "flex-start" }}
                  >
                    <Stack spacing={1.5}>
                      <Box
                        sx={{
                          width: 40,
                          height: 40,
                          borderRadius: 1.5,
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "center",
                          bgcolor: "action.hover",
                          color: "primary.main",
                        }}
                      >
                        <BookOpen size={20} aria-hidden />
                      </Box>
                      <Typography variant="subtitle1" component="span" sx={{ fontWeight: 700, display: "block" }}>
                        {kb.title}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        Browse articles and guides for {kb.title}.
                      </Typography>
                    </Stack>
                  </CardActionArea>
                </Card>
              </Grid>
            ))}
          </Grid>
        )}

        <Typography variant="h5" component="h2" sx={{ fontWeight: 700, mb: 2.5 }}>
          Recent articles
        </Typography>
        {recentLoading && (
          <Stack spacing={1.5}>
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} variant="rounded" height={56} />
            ))}
          </Stack>
        )}
        {recentFailed && (
          <QueryErrorState
            error={recent.error}
            message="Recent articles could not be loaded."
            onRetry={recent.isLoadMoreError ? recent.loadMore : recent.refetch}
          />
        )}
        {recentEmpty && !recent.hasMore && <EmptyState title="No articles yet" />}
        {recentEmpty && recent.hasMore && (
          <>
            <EmptyState
              title="No recent articles to show here"
              message="Load more to keep looking, or browse a knowledge base."
            />
            <LoadMoreButton onClick={recent.loadMore} loading={recent.isLoadingMore} />
          </>
        )}
        {recentArticles.length > 0 && (
          <ArticleList label="Recent articles">
            {recentArticles.map((a) => (
              <ArticleListItem key={a.id}>
                <ArticleCard
                  id={a.id}
                  title={a.title}
                  snippet={toSnippet(a.body)}
                  publishedOn={a.publishedOn}
                />
              </ArticleListItem>
            ))}
          </ArticleList>
        )}
      </Container>
    </>
  );
}
