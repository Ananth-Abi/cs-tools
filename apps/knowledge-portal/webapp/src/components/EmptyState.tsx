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

import { Stack, Typography } from "@wso2/oxygen-ui";
import { SearchX } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

interface EmptyStateProps {
  title: string;
  message?: string;
}

/** Nothing-to-show state for a list that loaded fine but is empty. */
export default function EmptyState({ title, message }: EmptyStateProps): JSX.Element {
  return (
    <Stack spacing={1} alignItems="center" sx={{ py: 6, textAlign: "center" }}>
      <SearchX size={32} strokeWidth={1.5} style={{ opacity: 0.6 }} aria-hidden />
      <Typography variant="subtitle1" component="p" fontWeight={600}>
        {title}
      </Typography>
      {message && (
        <Typography variant="body2" color="text.secondary">
          {message}
        </Typography>
      )}
    </Stack>
  );
}
