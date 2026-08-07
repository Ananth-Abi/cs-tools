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
  Paper,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TablePagination,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import RelativeTime from "@components/RelativeTime";
import QueryErrorState from "@components/QueryErrorState";
import SemanticChip from "@components/SemanticChip";
import ConversationTranscriptDialog from "@features/csm-projects/components/ConversationTranscriptDialog";
import { useSearchConversations } from "@features/csm-projects/api/useSearchConversations";
import type { BeConversationState, BeConversationView } from "@api/backend/types";

const COLUMN_COUNT = 4;
const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, DEFAULT_ROWS_PER_PAGE, 50];

const STATE_META: Record<BeConversationState, { label: string; role: "info" | "success" }> = {
  ACTIVE: { label: "Active", role: "info" },
  RESOLVED: { label: "Resolved", role: "success" },
};

interface ConversationsTabProps {
  projectId: string;
}

/**
 * Lists a project's chat sessions (`POST /conversations/search`), most
 * recently active first. Clicking a row opens the read-only transcript
 * (`ConversationTranscriptDialog`); a conversation that became a case links
 * through to it from there instead of duplicating that action inline here.
 */
export default function ConversationsTab({ projectId }: ConversationsTabProps): JSX.Element {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const [selected, setSelected] = useState<BeConversationView | null>(null);

  const { data, isLoading, isError, error } = useSearchConversations(projectId, {
    page,
    rowsPerPage,
  });
  const conversations = data?.conversations ?? [];
  const total = data?.total ?? 0;

  return (
    <>
      <Paper variant="outlined">
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Started</TableCell>
                <TableCell>Started by</TableCell>
                <TableCell>Messages</TableCell>
                <TableCell>State</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: 3 }).map((_, i) => (
                  <TableRow key={i}>
                    {Array.from({ length: COLUMN_COUNT }).map((__, c) => (
                      <TableCell key={c}>
                        <Skeleton variant="rounded" width="70%" height={18} />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={COLUMN_COUNT} align="center">
                    <QueryErrorState
                      message={
                        error instanceof Error && error.message.trim()
                          ? error.message
                          : "Failed to load conversations."
                      }
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : conversations.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={COLUMN_COUNT} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No chat sessions found for this project.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                conversations.map((c, i) => (
                  <TableRow
                    key={c.id ?? `${c.createdOn}-${i}`}
                    hover
                    onClick={() => setSelected(c)}
                    sx={{ cursor: "pointer" }}
                  >
                    <TableCell>
                      <RelativeTime iso={c.createdOn} />
                    </TableCell>
                    <TableCell>{c.createdBy || "—"}</TableCell>
                    <TableCell>{c.messageCount}</TableCell>
                    <TableCell>
                      {c.state ? (
                        <SemanticChip role={STATE_META[c.state].role} label={STATE_META[c.state].label} />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>

        <TablePagination
          component="div"
          count={total}
          page={page}
          onPageChange={(_, newPage) => setPage(newPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(e) => {
            setRowsPerPage(parseInt(e.target.value, 10));
            setPage(0);
          }}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          labelRowsPerPage="Conversations per page"
        />
      </Paper>

      {selected && (
        <ConversationTranscriptDialog conversation={selected} onClose={() => setSelected(null)} />
      )}
    </>
  );
}
