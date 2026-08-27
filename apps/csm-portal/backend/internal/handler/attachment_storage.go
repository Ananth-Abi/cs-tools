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

package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/sftpgo"
)

// shareTTL bounds how long a created SFTPGo share — and by extension the
// public download/inline-image URL handed to the caller — stays valid. Kept
// short because a share is minted fresh on every request that needs one; see
// AttachmentStorageHandler.CreateAttachmentShare's doc comment on why shares
// are never created eagerly.
const shareTTL = 5 * time.Minute

// jwtAssertionHeader mirrors the unexported constant of the same name in
// internal/middleware/auth.go — that package does not export it, so it is
// duplicated here rather than introducing a cross-package export for a
// single literal.
const jwtAssertionHeader = "x-jwt-assertion"

// sftpgoClient abstracts the SFTPGo operations used by AttachmentStorageHandler,
// allowing the handler to be tested without a live SFTPGo instance.
type sftpgoClient interface {
	MintToken(ctx context.Context, email, jwtAssertion string) (*sftpgo.Token, error)
	CreateShare(ctx context.Context, accessToken, storageKey string, ttl time.Duration) (string, error)
	PublicShareURL(shareID string) string
	BaseURL() string
}

// AttachmentStorageHandler implements the SFTPGo-backed attachment-storage
// endpoints: minting a short-lived SFTPGo access token before an upload, and
// creating a short-lived public download share for an already-stored
// attachment. It never touches attachment bytes itself — uploads and
// downloads go directly between the browser and SFTPGo using the
// credentials this handler mints; see package sftpgo's doc comment. Its
// routes are only registered (and therefore only reachable) when
// SFTPGO_ATTACHMENT_STORAGE_ENABLED is on — see cmd/server/main.go. The
// existing streaming attachment endpoints on CaseHandler are completely
// unaffected by this handler and by the flag.
type AttachmentStorageHandler struct {
	entity entityCaseClient
	sftpgo sftpgoClient
}

// NewAttachmentStorageHandler creates an AttachmentStorageHandler backed by
// the given entity and SFTPGo clients.
func NewAttachmentStorageHandler(entity entityCaseClient, sftpgo sftpgoClient) *AttachmentStorageHandler {
	return &AttachmentStorageHandler{entity: entity, sftpgo: sftpgo}
}

// uploadTokenResponse is the response body of
// POST /cases/{id}/attachments/upload-token.
type uploadTokenResponse struct {
	SftpgoAccessToken string          `json:"sftpgoAccessToken"`
	ExpiresAt         json.RawMessage `json:"expiresAt"`
	SftpgoBaseURL     string          `json:"sftpgoBaseUrl"`
}

// MintUploadToken handles POST /cases/{id}/attachments/upload-token. It mints
// a short-lived SFTPGo access token scoped to the caller's own identity (the
// gateway-validated email claim) for the browser to use directly against
// SFTPGo's chunked/TUS upload endpoint — this backend never sees the
// uploaded bytes. Requires write access to the target case, checked via the
// same guard CaseHandler.CreateCaseAttachment already applies (case exists
// and is not closed): a token is never minted for a case the caller could
// not otherwise attach a file to.
func (h *AttachmentStorageHandler) MintUploadToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" || !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	if !h.canWriteCase(w, r, caseID) {
		return
	}

	jwtAssertion := r.Header.Get(jwtAssertionHeader)
	if jwtAssertion == "" {
		// The Auth middleware already rejects any request without this
		// header before it reaches here, so this is not a real caller path —
		// fail closed rather than mint a token with an empty credential.
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	token, err := h.sftpgo.MintToken(r.Context(), user.Email, jwtAssertion)
	if err != nil {
		slog.ErrorContext(r.Context(), "sftpgo MintToken failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		writeError(w, http.StatusBadGateway, "Failed to obtain an upload token.")
		return
	}

	writeJSONValue(w, http.StatusOK, uploadTokenResponse{
		SftpgoAccessToken: token.AccessToken,
		ExpiresAt:         token.ExpiresAt,
		SftpgoBaseURL:     h.sftpgo.BaseURL(),
	})
}

// attachmentMeta is the subset of the entity service's Attachment fields this
// handler needs (see openapi.yaml's Attachment schema) plus storageKey.
//
// Assumption flagged: storageKey does not exist on the entity service's
// Attachment schema today — every existing attachment is a base64 payload
// the entity service stores itself, with no notion of an external storage
// path. This field is assumed to be added by a corresponding entity-service
// change (out of scope for this layer/PR) that populates it with the SFTPGo
// path an attachment was uploaded to, only when it was created via the
// SFTPGo-backed upload path. A missing/empty storageKey here is treated as
// "not shareable" rather than an error — see StorageKey's nil check below —
// so an attachment stored the old way fails cleanly instead of panicking.
type attachmentMeta struct {
	ReferenceID   string  `json:"referenceId"`
	ReferenceType string  `json:"referenceType"`
	StorageKey    *string `json:"storageKey"`
}

// shareResponse is the response body of POST /attachments/{id}/share.
type shareResponse struct {
	ShareURL string `json:"shareUrl"`
}

// CreateAttachmentShare handles POST /attachments/{id}/share. It creates a
// fresh, short-lived (shareTTL) SFTPGo public share for one attachment's
// stored file and returns its public URL.
//
// This must be called lazily — once per attachment id, only when that
// specific attachment is actually opened or an inline image is actually
// rendered — never eagerly for a whole attachment list or comment thread: a
// share is a real SFTPGo object with its own lifecycle, and creating one for
// every attachment on every list/search response would mint SFTPGo shares
// nobody asked to open.
func (h *AttachmentStorageHandler) CreateAttachmentShare(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	attachmentID := r.PathValue("id")
	if attachmentID == "" || !uuidRe.MatchString(attachmentID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	raw, err := h.entity.GetCaseAttachment(r.Context(), attachmentID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCaseAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve attachment.")
		return
	}
	var meta attachmentMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		slog.ErrorContext(r.Context(), "failed to parse attachment metadata", "userID", user.UserID, "attachmentID", attachmentID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	// Read-access check: mirrors the (lack of) additional case-state gating
	// on GetCaseAttachmentContent and SearchCaseAttachments today — those
	// endpoints impose no restriction beyond the entity service's own
	// GetCase access control (a 403/404 from upstream decides who can see
	// what). Reused here, unchanged, for the read path.
	if meta.ReferenceType == "case" && meta.ReferenceID != "" {
		if !h.canReadCase(w, r, meta.ReferenceID) {
			return
		}
	}

	if meta.StorageKey == nil || *meta.StorageKey == "" {
		writeError(w, http.StatusConflict, ErrMsgAttachmentNotShareable)
		return
	}

	jwtAssertion := r.Header.Get(jwtAssertionHeader)
	if jwtAssertion == "" {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	token, err := h.sftpgo.MintToken(r.Context(), user.Email, jwtAssertion)
	if err != nil {
		slog.ErrorContext(r.Context(), "sftpgo MintToken failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
		writeError(w, http.StatusBadGateway, "Failed to create a share for this attachment.")
		return
	}

	shareID, err := h.sftpgo.CreateShare(r.Context(), token.AccessToken, *meta.StorageKey, shareTTL)
	if err != nil {
		slog.ErrorContext(r.Context(), "sftpgo CreateShare failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
		writeError(w, http.StatusBadGateway, "Failed to create a share for this attachment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, shareResponse{ShareURL: h.sftpgo.PublicShareURL(shareID)})
}

// canWriteCase mirrors CaseHandler.CreateCaseAttachment's closed-case guard —
// the same check that gates whether a case may receive a new attachment
// today — reused here so an upload token is never minted for a case an
// upload could not proceed against anyway.
func (h *AttachmentStorageHandler) canWriteCase(w http.ResponseWriter, r *http.Request, caseID string) bool {
	current, err := h.entity.GetCase(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed during attachment-storage write guard", "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to validate case access.")
		return false
	}
	var currentCase struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(current, &currentCase); err != nil {
		slog.ErrorContext(r.Context(), "failed to parse case state for attachment-storage write guard", "caseID", caseID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return false
	}
	if currentCase.State == "closed" {
		writeError(w, http.StatusConflict, ErrMsgAttachmentOnClosedCase)
		return false
	}
	return true
}

// canReadCase confirms the caller can view the target case at all — mirrors
// the (lack of) additional gating on GetCaseAttachmentContent and
// SearchCaseAttachments today: those endpoints impose no case-state
// restriction and rely entirely on the entity service's own GetCase access
// control (a 403/404 from upstream) to decide who can see what.
func (h *AttachmentStorageHandler) canReadCase(w http.ResponseWriter, r *http.Request, caseID string) bool {
	if _, err := h.entity.GetCase(r.Context(), caseID); err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed during attachment-storage read guard", "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to validate case access.")
		return false
	}
	return true
}
