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
import type { BeDashboardListItem, BeTeam } from "@api/backend/types";

interface TeamsSearchPayload {
  filters?: { family?: string };
  pagination: { offset: number; limit: number };
}

interface TeamsSearchResponse {
  teams: BeTeam[];
}

/**
 * Maps a dashboard's `type` to the team `family` its picker should be
 * scoped to — `cre` dashboards offer only `cre-abt` teams, `sre` only
 * `sre-abt`. `cs` and an untyped (legacy) dashboard aren't team-based at
 * all, so this only matters for dashboards where `isTeamBased` is true.
 */
export function abtFamilyForDashboardType(
  type: BeDashboardListItem["type"],
): string | undefined {
  switch (type) {
    case "cre":
      return "cre-abt";
    case "sre":
      return "sre-abt";
    default:
      return undefined;
  }
}

/**
 * The rough inverse of `abtFamilyForDashboardType`: maps a team's own
 * `family` (e.g. `"sre-abt"`, `"cre"`) to the dashboard `type` its default
 * selection should prefer — any family starting with `"cre"` to the `cre`
 * type, any starting with `"sre"` to `sre`. Returns `undefined` for a
 * family that doesn't start with either prefix (or an unresolved
 * family), which callers must treat as "no type preference," not as a
 * dashboard-selection dead end — see `preferredEntry` in
 * `CsmDashboardPage.tsx`.
 */
export function dashboardTypeForTeamFamily(
  family: string | undefined,
): BeDashboardListItem["type"] {
  if (!family) return undefined;
  if (family.startsWith("cre")) return "cre";
  if (family.startsWith("sre")) return "sre";
  return undefined;
}

/**
 * Every team from `POST /teams/search`, for the team selector a team-based
 * dashboard shows alongside the dashboard switcher (see
 * `AbtDashboardHeader`), and for `CsmDashboardPage` to resolve the selected
 * team's own `creGroupId`/`sreGroupId` — the values substituted for the
 * `__current_team__` filter placeholder in a `creTeam`/`sreTeam` filter
 * entry, respectively (see `teamFilterPlaceholder.ts`). `family`, when given,
 * scopes the result to that team family only (see
 * `abtFamilyForDashboardType`) — omitted, every team in the registry is
 * returned regardless of family.
 */
export function useTeams(
  enabled: boolean,
  family?: string,
): UseQueryResult<BeTeam[], Error> {
  const api = useBackendApi();

  return useQuery<BeTeam[], Error>({
    queryKey: [ApiQueryKeys.CSM_TEAMS, family ?? null],
    queryFn: async (): Promise<BeTeam[]> => {
      const res = await api.post<TeamsSearchPayload, TeamsSearchResponse>(
        "/teams/search",
        {
          filters: family ? { family } : undefined,
          pagination: { offset: 0, limit: 100 },
        },
      );
      return res.teams ?? [];
    },
    enabled,
    staleTime: 5 * 60_000,
  });
}
