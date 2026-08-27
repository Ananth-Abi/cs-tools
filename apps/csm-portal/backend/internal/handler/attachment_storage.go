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
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path"
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

// uploadShareTTL bounds how long a write-scoped upload share (minted by
// MintUploadToken) stays valid. Deliberately much longer than shareTTL: an
// upload share has to stay open for the entire duration of the browser's
// direct TUS upload to SFTPGo — including retries/resumes of a large or
// slow file — not just long enough to redeem a single GET. 45 minutes is a
// documented, reasonable choice rather than a value verified against any
// real-world upload-duration data; revisit if large attachments start
// failing with an expired-share error near the end of a long upload.
const uploadShareTTL = 45 * time.Minute

// jwtAssertionHeader mirrors the unexported constant of the same name in
// internal/middleware/auth.go — that package does not export it, so it is
// duplicated here rather than introducing a cross-package export for a
// single literal.
const jwtAssertionHeader = "x-jwt-assertion"

// sftpgoClient abstracts the SFTPGo operations used by AttachmentStorageHandler,
// allowing the handler to be tested without a live SFTPGo instance.
type sftpgoClient interface {
	MintToken(ctx context.Context, email, jwtAssertion string) (*sftpgo.Token, error)
	CreateShare(ctx context.Context, accessToken, storageKey string, scope int, ttl time.Duration) (string, error)
	PublicShareURL(shareID string) string
	BaseURL() string
}

// AttachmentStorageHandler implements the SFTPGo-backed attachment-storage
// endpoints: minting a write-scoped SFTPGo share before an upload, and
// creating a short-lived read-scoped public download share for an
// already-stored attachment. It never touches attachment bytes itself —
// uploads and downloads go directly between the browser and SFTPGo, and the
// browser never sees a bearer credential of any kind: a Share is scoped to
// exactly one storage path and one direction (read or write), so the worst
// a leaked share id can do is read or write that single path — see package
// sftpgo's doc comment. Its routes are only registered (and therefore only
// reachable) when SFTPGO_ATTACHMENT_STORAGE_ENABLED is on — see
// cmd/server/main.go. The existing streaming attachment endpoints on
// CaseHandler are completely unaffected by this handler and by the flag.
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
	// ShareID is a write-scoped, passwordless SFTPGo share id, restricted to
	// StorageKey's parent directory (see the CreateShare call in
	// MintUploadToken for why it is the directory and not the file itself —
	// SFTPGo's shares-chunked-uploads endpoint always resolves the TUS
	// "path" metadata relative to the share's own scoped path, confirmed
	// empirically against a real instance; scoping the share to the exact
	// file, rather than its directory, made every upload fail). The frontend
	// embeds this id as the "share_id" key in the TUS Upload-Metadata header
	// it sends to SFTPGo's POST /shares-chunked-uploads, and MUST send only
	// StorageKey's final path segment (everything after the last "/") as
	// the "path" key — NOT the full StorageKey — since the share's own root
	// already covers the directory portion; sending the full StorageKey as
	// "path" would double it up (e.g. ".../case-1/case-1/<id>") and fail.
	// That share id is the entire credential; no bearer token or
	// Authorization header is ever involved in the upload. The frontend must
	// also set "mkdir_parents" to "true" in the same Upload-Metadata header:
	// this directory is not guaranteed to already exist (e.g. a case's first
	// attachment ever), and SFTPGo does not create it implicitly otherwise.
	// The share's own server-side expiry governs how long the upload window
	// stays open; the frontend does not need that value, it either works or
	// the share is gone.
	ShareID       string `json:"shareId"`
	SftpgoBaseURL string `json:"sftpgoBaseUrl"`
	// StorageKey is the exact SFTPGo path the uploaded file must end up at,
	// and must later be sent back unchanged as
	// CreateAttachmentRequest.storageKey when the frontend creates the
	// attachment metadata row. Minted here, server-side, rather than left
	// for the frontend to invent, so the id embedded in it is guaranteed to
	// match no other attachment. See buildStorageKey for the path
	// convention, and ShareID's doc comment above for how the frontend must
	// derive the TUS upload's "path" metadata from this value.
	StorageKey string `json:"storageKey"`
}

// MintUploadToken handles POST /cases/{id}/attachments/upload-token. It
// mints a write-scoped, passwordless SFTPGo share restricted to a single,
// freshly generated storage path, for the browser to use directly against
// SFTPGo's share-authenticated chunked/TUS upload endpoint
// (POST /shares-chunked-uploads) — this backend never sees the uploaded
// bytes, and no bearer credential of any kind reaches the browser: a
// write-scoped share can do nothing but write to the one path it names.
// Requires write access to the target case, checked via the same guard
// CaseHandler.CreateCaseAttachment already applies (case exists and is not
// closed): a share is never minted for a case the caller could not otherwise
// attach a file to.
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

	projectID, ok := h.canWriteCase(w, r, caseID)
	if !ok {
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

	// MintToken is still needed here even though the resulting access token
	// is never returned to the frontend: this backend still has to
	// authenticate its own server-side call to POST /api/v2/user/shares
	// below, on the caller's behalf.
	token, err := h.sftpgo.MintToken(r.Context(), user.Email, jwtAssertion)
	if err != nil {
		slog.ErrorContext(r.Context(), "sftpgo MintToken failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		writeError(w, http.StatusBadGateway, "Failed to obtain an upload token.")
		return
	}

	// Generated here, server-side, rather than left for the frontend to
	// invent: the frontend has no way to guarantee id uniqueness or apply the
	// storage-key convention, and both are this backend's responsibility.
	attachmentID := newAttachmentID()
	storageKey := buildStorageKey(projectID, caseID, attachmentID)

	// The share is scoped to storageKey's parent DIRECTORY, not storageKey
	// itself — confirmed empirically against a real SFTPGo instance that a
	// write-scope share used with POST /shares-chunked-uploads always
	// resolves the TUS "path" metadata relative to the share's own path, so
	// a share scoped to the exact file (rather than its directory) makes
	// every upload against it fail with "unable to write to file". Scoping
	// to the directory still keeps the share unable to touch any other
	// case's files: buildStorageKey gives every case (and, when known,
	// every project) its own directory, so the worst a leaked share id can
	// do is write within this one case's attachment directory. See
	// uploadTokenResponse.ShareID's doc comment for the corresponding
	// frontend-side contract.
	shareDir := path.Dir(storageKey)
	shareID, err := h.sftpgo.CreateShare(r.Context(), token.AccessToken, shareDir, sftpgo.ShareScopeWrite, uploadShareTTL)
	if err != nil {
		slog.ErrorContext(r.Context(), "sftpgo CreateShare (write) failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		writeError(w, http.StatusBadGateway, "Failed to obtain an upload token.")
		return
	}

	writeJSONValue(w, http.StatusOK, uploadTokenResponse{
		ShareID:       shareID,
		SftpgoBaseURL: h.sftpgo.BaseURL(),
		StorageKey:    storageKey,
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

	shareID, err := h.sftpgo.CreateShare(r.Context(), token.AccessToken, *meta.StorageKey, sftpgo.ShareScopeRead, shareTTL)
	if err != nil {
		slog.ErrorContext(r.Context(), "sftpgo CreateShare (read) failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
		writeError(w, http.StatusBadGateway, "Failed to create a share for this attachment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, shareResponse{ShareURL: h.sftpgo.PublicShareURL(shareID)})
}

// canWriteCase mirrors CaseHandler.CreateCaseAttachment's closed-case guard —
// the same check that gates whether a case may receive a new attachment
// today — reused here so an upload token is never minted for a case an
// upload could not proceed against anyway. projectID is the case's
// projectId as reported by the entity service (see domain.Case.ProjectID),
// used by MintUploadToken to build the storage key; it is "" when the
// upstream response omits it, which buildStorageKey treats as "no project
// concept for this case" rather than an error.
func (h *AttachmentStorageHandler) canWriteCase(w http.ResponseWriter, r *http.Request, caseID string) (projectID string, ok bool) {
	current, err := h.entity.GetCase(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed during attachment-storage write guard", "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to validate case access.")
		return "", false
	}
	var currentCase struct {
		State     string `json:"state"`
		ProjectID string `json:"projectId"`
	}
	if err := json.Unmarshal(current, &currentCase); err != nil {
		slog.ErrorContext(r.Context(), "failed to parse case state for attachment-storage write guard", "caseID", caseID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return "", false
	}
	if currentCase.State == "closed" {
		writeError(w, http.StatusConflict, ErrMsgAttachmentOnClosedCase)
		return "", false
	}
	return currentCase.ProjectID, true
}

// newAttachmentID generates a random UUID v4 for a not-yet-created
// attachment. Mirrors middleware.newCorrelationID's approach (that helper is
// unexported in a different package, so it is duplicated here rather than
// exporting a single-purpose helper across a package boundary for it).
func newAttachmentID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("attachment_storage: failed to read random bytes: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// buildStorageKey computes the SFTPGo path an attachment's bytes live under:
// "/attachments/project-<projectId>/cases/<caseId>/<attachmentId>". SFTPGo
// permissions are granted per project, so the project segment is
// load-bearing whenever a project is known.
//
// projectID is "" when the case's own record carries no project reference.
// Cases in this Postgres/CSM-native data source are NOT guaranteed to have a
// project: domain.Case.ProjectID exists on the schema, but nothing in
// entity-service enforces it is always populated for a CSM-native case
// (unlike ServiceNow-sourced cases, which are always project-scoped). Rather
// than block minting a token over a missing project reference, this falls
// back to a project-less path shape,
// "/attachments/cases/<caseId>/<attachmentId>", which still uniquely
// identifies the file. This fallback path cannot be granted SFTPGo
// permissions per-project the way the documented convention can; it is
// accepted here as a deliberate, narrower scope (case-only) rather than a
// blocker, and should be revisited if/when CSM-native cases gain a
// guaranteed project reference.
func buildStorageKey(projectID, caseID, attachmentID string) string {
	if projectID == "" {
		return fmt.Sprintf("/attachments/cases/%s/%s", caseID, attachmentID)
	}
	return fmt.Sprintf("/attachments/project-%s/cases/%s/%s", projectID, caseID, attachmentID)
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
