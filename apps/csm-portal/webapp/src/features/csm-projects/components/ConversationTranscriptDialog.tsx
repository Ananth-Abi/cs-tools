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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Skeleton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { Link as RouterLink } from "react-router";
import CsmCaseCommentBubble from "@features/csm-cases/components/CsmCaseCommentBubble";
import { useGetCsmConversationMessages } from "@features/csm-cases/api/useCsmConversationMessages";
import type { BeConversationView } from "@api/backend/types";

interface ConversationTranscriptDialogProps {
  conversation: BeConversationView;
  onClose: () => void;
}

/**
 * Read-only transcript of a single chat session, opened from a project's
 * Conversations tab. Reuses `useGetCsmConversationMessages` and
 * `CsmCaseCommentBubble` — the same hook and bubble the case-detail activity
 * feed uses for the Novera transcript a case originated from — so message
 * rendering (sanitisation, author role, etc.) stays identical wherever a
 * transcript is shown.
 */
export default function ConversationTranscriptDialog({
  conversation,
  onClose,
}: ConversationTranscriptDialogProps): JSX.Element {
  const { data: messages, isLoading, isError } = useGetCsmConversationMessages(
    conversation.id,
  );

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        Chat session {conversation.number ? `· ${conversation.number}` : ""}
      </DialogTitle>
      <DialogContent dividers sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        {isLoading ? (
          <Stack gap={1.5}>
            <Skeleton variant="rounded" height={64} />
            <Skeleton variant="rounded" height={64} />
            <Skeleton variant="rounded" height={64} />
          </Stack>
        ) : isError ? (
          <Typography variant="body2" color="error">
            Could not load this conversation.
          </Typography>
        ) : !messages || messages.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            No messages in this conversation.
          </Typography>
        ) : (
          <Stack gap={1.5}>
            {messages.map((message) => (
              <CsmCaseCommentBubble key={message.id} comment={message} />
            ))}
          </Stack>
        )}
      </DialogContent>
      <DialogActions>
        {conversation.case?.id && (
          <Box sx={{ mr: "auto" }}>
            <Button component={RouterLink} to={`/cases/${conversation.case.id}`} onClick={onClose}>
              View case
            </Button>
          </Box>
        )}
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}
