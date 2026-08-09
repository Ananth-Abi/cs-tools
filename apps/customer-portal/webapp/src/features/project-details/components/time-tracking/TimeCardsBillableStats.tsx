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

import { Box, Card, CardContent, Typography, Skeleton } from "@wso2/oxygen-ui";
import { Clock } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

import useGetTimeCardsStats from "@features/usage-metrics/api/useGetTimeCardsStats";
import { formatServiceHoursDecimalCompact } from "@features/project-details/utils/projectDetails";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import type { TimeCardsBillableStatsProps } from "@features/project-details/types/projectDetailsComponents";

/**
 * TimeCardsBillableStats displays billable and non-billable hours for the currently
 * applied time card date range.
 *
 * @param {TimeCardsBillableStatsProps} props - Project ID and applied date range.
 * @returns {JSX.Element} The rendered TimeCardsBillableStats component.
 */
export default function TimeCardsBillableStats({
  projectId,
  startDate,
  endDate,
}: TimeCardsBillableStatsProps): JSX.Element {
  const {
    data,
    isLoading,
    isError,
  } = useGetTimeCardsStats({ projectId, startDate, endDate });

  return (
    <Box
      sx={{
        display: "grid",
        gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
        gap: 2,
        mb: 2,
      }}
    >
      <Card sx={{ p: 2 }}>
        <CardContent sx={{ p: 0, "&:last-child": { pb: 0 } }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              mb: 1,
            }}
          >
            <Typography variant="body2" color="text.secondary">
              Billable Hours
            </Typography>
            <Box
              component="span"
              sx={{ color: "info.main", display: "inline-flex" }}
            >
              <Clock size={16} />
            </Box>
          </Box>
          {isLoading ? (
            <Skeleton variant="text" width="60%" height={32} />
          ) : isError ? (
            <ErrorIndicator entityName="billable hours" />
          ) : (
            <Typography variant="h5">
              {formatServiceHoursDecimalCompact(data?.billableHours)}
            </Typography>
          )}
        </CardContent>
      </Card>

      <Card sx={{ p: 2 }}>
        <CardContent sx={{ p: 0, "&:last-child": { pb: 0 } }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              mb: 1,
            }}
          >
            <Typography variant="body2" color="text.secondary">
              Non-Billable Hours
            </Typography>
            <Box
              component="span"
              sx={{ color: "success.main", display: "inline-flex" }}
            >
              <Clock size={16} />
            </Box>
          </Box>
          {isLoading ? (
            <Skeleton variant="text" width="60%" height={32} />
          ) : isError ? (
            <ErrorIndicator entityName="non-billable hours" />
          ) : (
            <Typography variant="h5">
              {formatServiceHoursDecimalCompact(data?.nonBillableHours)}
            </Typography>
          )}
        </CardContent>
      </Card>
    </Box>
  );
}
