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
  Button,
  Card,
  Chip,
  IconButton,
  Skeleton,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { LayoutGrid, Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX } from "react";
import { useNavigate } from "react-router";
import { useDashboardList } from "@features/csm-dashboard/api/useDashboardList";
import {
  deleteDashboardDraft,
  newDraftId,
  useDashboardDrafts,
} from "@features/csm-admin/dashboards/utils/dashboardDraftsStorage";

/**
 * Admin-only landing page for the dashboard builder: every deployed
 * dashboard (`GET /dashboards`), each openable for edit, plus any local
 * draft that hasn't (yet) been opened from — or matched to — a deployed
 * one. There is no dashboard CRUD API; "Edit" always opens the builder
 * against a `localStorage` draft, seeded from the live dashboard the first
 * time it's opened (see `CsmDashboardBuilderEditorPage`).
 */
export default function CsmDashboardBuilderListPage(): JSX.Element {
  const navigate = useNavigate();
  const { data: dashboards, isLoading, isError } = useDashboardList();
  const drafts = useDashboardDrafts();

  const liveIds = useMemo(() => new Set((dashboards ?? []).map((d) => d.id)), [dashboards]);
  const draftIds = useMemo(() => new Set(drafts.map((d) => d.id)), [drafts]);
  // Drafts that don't (yet) correspond to any deployed dashboard — either a
  // brand-new dashboard that's never been deployed, or a draft whose
  // deployed source has since been removed from the registry.
  const orphanDrafts = useMemo(() => drafts.filter((d) => !liveIds.has(d.id)), [drafts, liveIds]);

  const handleCreate = (): void => {
    navigate(`/admin/dashboards/${newDraftId()}`);
  };

  const handleDiscardDraft = (id: string): void => {
    deleteDashboardDraft(id);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="body2" color="text.secondary">
          Build or adjust a dashboard's widgets, then hand the exported JSON to a maintainer to
          deploy — this builder never writes to the live registry itself.
        </Typography>
        <Button variant="contained" startIcon={<Plus size={16} />} onClick={handleCreate}>
          Create new dashboard
        </Button>
      </Box>

      {isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load the dashboard list.
        </Typography>
      ) : isLoading ? (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          {Array.from({ length: 3 }, (_, i) => (
            <Skeleton key={i} variant="rounded" height={64} />
          ))}
        </Box>
      ) : (dashboards ?? []).length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No dashboards are registered in this deployment yet.
        </Typography>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          {(dashboards ?? []).map((d) => (
            <Card
              key={d.id}
              variant="outlined"
              sx={{
                p: 2,
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 2,
              }}
            >
              <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, minWidth: 0 }}>
                <LayoutGrid size={18} />
                <Box sx={{ minWidth: 0 }}>
                  <Typography variant="body1" sx={{ fontWeight: 600 }} noWrap>
                    {d.displayName}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {d.id}
                  </Typography>
                </Box>
                {d.isDefault && <Chip size="small" label="Default" />}
                {d.isTeamBased && <Chip size="small" label="Team-based" variant="outlined" />}
                {d.type && <Chip size="small" label={d.type.toUpperCase()} variant="outlined" />}
                {draftIds.has(d.id) && (
                  <Tooltip title="A local draft for this dashboard has unsaved-to-deployment changes">
                    <Chip size="small" color="warning" label="Local draft" />
                  </Tooltip>
                )}
              </Box>
              <Button
                variant="outlined"
                size="small"
                onClick={() => navigate(`/admin/dashboards/${d.id}`)}
              >
                Edit
              </Button>
            </Card>
          ))}
        </Box>
      )}

      {orphanDrafts.length > 0 && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Typography variant="subtitle2">
            Local drafts not yet deployed
          </Typography>
          {orphanDrafts.map((d) => (
            <Card
              key={d.id}
              variant="outlined"
              sx={{
                p: 2,
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 2,
              }}
            >
              <Box sx={{ minWidth: 0 }}>
                <Typography variant="body1" sx={{ fontWeight: 600 }} noWrap>
                  {d.displayName || "Untitled dashboard"}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  Saved locally {new Date(d.updatedAt).toLocaleString()}
                </Typography>
              </Box>
              <Box sx={{ display: "flex", gap: 1 }}>
                <Button
                  variant="outlined"
                  size="small"
                  onClick={() => navigate(`/admin/dashboards/${d.id}`)}
                >
                  Continue editing
                </Button>
                <Tooltip title="Discard this local draft">
                  <IconButton
                    size="small"
                    aria-label={`Discard draft ${d.displayName || d.id}`}
                    onClick={() => handleDiscardDraft(d.id)}
                  >
                    <Trash2 size={16} />
                  </IconButton>
                </Tooltip>
              </Box>
            </Card>
          ))}
        </Box>
      )}
    </Box>
  );
}
