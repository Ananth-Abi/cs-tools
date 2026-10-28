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
  Breadcrumbs,
  Button,
  Chip,
  Container,
  Divider,
  Grid,
  Link as OxygenLink,
  Paper,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link, useParams } from "react-router";
import { isClientError, isNotFoundError } from "@api/backend/client";
import { isValidId } from "@api/backend/types";
import type { KBArticle, RelatedArticle } from "@api/backend/types";
import { useArticle, useKnowledgeBases, useRelatedArticles } from "@api/useKb";
import LoadingState from "@components/LoadingState";
import NotFoundState from "@components/NotFoundState";
import QueryErrorState from "@components/QueryErrorState";
import ArticleCard from "@features/kb/components/ArticleCard";
import ArticleList, { ArticleListItem } from "@features/kb/components/ArticleList";
import { useDocumentTitle } from "@hooks/useDocumentTitle";
import { accessibleLinkSx } from "@config/a11yThemeOverrides";
import { browseUrl } from "@utils/browseUrl";
import { formatDate } from "@utils/formatDate";
import { sanitizeArticleHtml } from "@utils/sanitizeHtml";

const BODY_SX = {
  maxWidth: "80ch",
  fontSize: "1.0625rem",
  lineHeight: 1.75,
  color: "text.primary",
  overflowWrap: "anywhere",
  "& > *:first-of-type": { mt: 0 },
  "& h1, & h2, & h3, & h4": {
    fontWeight: 600,
    lineHeight: 1.3,
    mt: 5,
    mb: 1.5,
    "& a.headerlink, & .headerlink": { display: "none" },
  },
  "& h1": { fontSize: "1.75rem" },
  "& h2": { fontSize: "1.4rem" },
  "& h3": { fontSize: "1.2rem" },
  "& h4": { fontSize: "1.05rem" },
  "& p": { my: 2 },
  "& ul, & ol": { my: 2, pl: 3 },
  "& li": { mb: 1 },
  "& li::marker": { color: "text.secondary" },
  "& a": accessibleLinkSx,
  "& code": {
    px: 0.75,
    py: 0.25,
    borderRadius: 0.75,
    bgcolor: "action.hover",
    fontFamily: "monospace",
    fontSize: "0.9em",
  },
  "& pre": {
    overflowX: "auto",
    p: 2,
    my: 3,
    borderRadius: 1,
    bgcolor: "action.hover",
    border: 1,
    borderColor: "divider",
    "& code": { bgcolor: "transparent", px: 0, py: 0 },
  },
  "& blockquote": {
    my: 3,
    ml: 0,
    pl: 2,
    borderLeft: 3,
    borderColor: "divider",
    color: "text.secondary",
  },
  "& img": { maxWidth: "100%", height: "auto", borderRadius: 1, my: 2 },
  "& table": { width: "100%", borderCollapse: "collapse", my: 3, fontSize: "0.9375rem" },
  "& td, & th": { border: 1, borderColor: "divider", p: 1.25 },
  "& th": { bgcolor: "action.hover", fontWeight: 600, textAlign: "left" },
} as const;

export default function KBArticlePage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const idIsValid = isValidId(id);

  const articleQuery = useArticle(id);
  const { data: article } = articleQuery;
  const { data: kbData } = useKnowledgeBases();
  const { data: related } = useRelatedArticles(article?.id);

  const knowledgeBaseTitle = article
    ? kbData?.knowledgeBases.find((kb) => kb.id === article.knowledgeBaseId)?.title
    : undefined;

  const notFound = !idIsValid || isNotFoundError(articleQuery.error);

  // An id the backend would reject never reaches it: it cannot exist.
  if (notFound) {
    return (
      <NotFoundState
        title="Article not found"
        message="This article may have been retired, or the link may be incorrect."
      />
    );
  }

  if (articleQuery.isLoading) return <LoadingState label="Loading article" py={10} />;

  if (articleQuery.isError || !article) {
    return (
      <Container maxWidth="lg" sx={{ py: 8 }}>
        <ArticleErrorTitle />
        <QueryErrorState
          headingComponent="h1"
          error={articleQuery.error}
          message="The article could not be loaded."
          onRetry={isClientError(articleQuery.error) ? undefined : () => void articleQuery.refetch()}
        />
        <Stack alignItems="center">
          <Button component={Link} to="/" variant="outlined" startIcon={<ArrowLeft size={16} aria-hidden />}>
            Back to knowledge base
          </Button>
        </Stack>
      </Container>
    );
  }

  return (
    <ArticleView
      article={article}
      knowledgeBaseTitle={knowledgeBaseTitle}
      relatedArticles={related?.articles ?? []}
    />
  );
}

/** Title for the load-failure state. Rendered only in that state, so nothing competes with it. */
function ArticleErrorTitle(): null {
  useDocumentTitle("Article could not be loaded");
  return null;
}

interface ArticleViewProps {
  article: KBArticle;
  knowledgeBaseTitle?: string;
  relatedArticles: RelatedArticle[];
}

function ArticleView({ article, knowledgeBaseTitle, relatedArticles }: ArticleViewProps): JSX.Element {
  useDocumentTitle(article.title);
  const published = formatDate(article.publishedOn);

  return (
    <Container maxWidth="xl" sx={{ py: 5 }}>
      <Breadcrumbs aria-label="Breadcrumb" sx={{ mb: 3 }}>
        <OxygenLink component={Link} to="/" underline="hover" color="inherit">
          Knowledge Base
        </OxygenLink>
        {knowledgeBaseTitle && (
          <OxygenLink
            component={Link}
            to={browseUrl({ knowledgeBaseId: article.knowledgeBaseId })}
            underline="hover"
            color="inherit"
          >
            {knowledgeBaseTitle}
          </OxygenLink>
        )}
        <Typography color="text.primary" variant="body2" aria-current="page">
          {article.title}
        </Typography>
      </Breadcrumbs>

      <Grid container spacing={5}>
        <Grid size={{ xs: 12, md: 9 }}>
          <Paper component="article" variant="outlined" sx={{ p: { xs: 3, md: 5 }, borderRadius: 2 }}>
            {article.number && (
              <Typography variant="body2" color="text.secondary" sx={{ fontFamily: "monospace", mb: 1 }}>
                {article.number}
              </Typography>
            )}
            <Typography variant="h4" component="h1" sx={{ fontWeight: 700, mb: 1.5 }}>
              {article.title}
            </Typography>
            <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" sx={{ color: "text.secondary" }}>
              {knowledgeBaseTitle && (
                <Chip
                  size="small"
                  variant="outlined"
                  label={knowledgeBaseTitle}
                  component={Link}
                  to={browseUrl({ knowledgeBaseId: article.knowledgeBaseId })}
                  clickable
                />
              )}
              {published && article.publishedOn && (
                <Typography variant="body2">
                  Published{" "}
                  <time dateTime={article.publishedOn}>{published}</time>
                </Typography>
              )}
            </Stack>
            <Divider sx={{ my: 3 }} />
            <Box
              sx={BODY_SX}
              dangerouslySetInnerHTML={{ __html: sanitizeArticleHtml(article.body) }}
            />
          </Paper>
        </Grid>

        <Grid size={{ xs: 12, md: 3 }}>
          {relatedArticles.length > 0 && (
            <Box component="aside" aria-labelledby="related-heading">
              <Typography id="related-heading" variant="subtitle2" component="h2" sx={{ fontWeight: 600, mb: 1.5 }}>
                Related articles
              </Typography>
              <ArticleList label="Related articles">
                {relatedArticles.map((a) => (
                  <ArticleListItem key={a.id}>
                    <ArticleCard id={a.id} title={a.title} />
                  </ArticleListItem>
                ))}
              </ArticleList>
            </Box>
          )}
        </Grid>
      </Grid>
    </Container>
  );
}
