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

import { Card, CardActionArea, Stack, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { Link } from "react-router";
import { articleUrl } from "@utils/browseUrl";
import { formatDate } from "@utils/formatDate";

interface ArticleCardProps {
  id: string;
  title: string;
  /** Plain-text excerpt. Omit for a compact title-only card. */
  snippet?: string;
  publishedOn?: string | null;
}

/**
 * One article as a real link (CardActionArea renders an anchor), so
 * middle-click, "open in new tab" and screen-reader link lists all work.
 * Render inside a list item.
 */
export default function ArticleCard({
  id,
  title,
  snippet,
  publishedOn,
}: ArticleCardProps): JSX.Element {
  const published = formatDate(publishedOn);
  return (
    <Card variant="outlined">
      <CardActionArea
        component={Link}
        to={articleUrl(id)}
        sx={{ px: 2.5, py: snippet ? 2.5 : 1.75, alignItems: "flex-start" }}
      >
        <Typography variant="subtitle1" component="span" sx={{ fontWeight: 600, display: "block" }}>
          {title}
        </Typography>
        {snippet && (
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            {snippet}
          </Typography>
        )}
        {published && publishedOn && (
          <Stack direction="row" sx={{ mt: 1, color: "text.secondary" }}>
            <Typography variant="caption" component="time" dateTime={publishedOn}>
              {published}
            </Typography>
          </Stack>
        )}
      </CardActionArea>
    </Card>
  );
}
