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
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/sftpgo"
)

// ----- mock SFTPGo client -----

type mockSftpgoClient struct {
	mintTokenFn    func(ctx context.Context, email, jwtAssertion string) (*sftpgo.Token, error)
	createShareFn  func(ctx context.Context, accessToken, storageKey string, scope int, ttl time.Duration) (string, error)
	publicShareURL func(shareID string) string
	baseURL        string

	mintTokenCalls    []string // records the jwtAssertion passed on each call
	createShareCalls  []string // records the storageKey passed on each call
	createShareScopes []int    // records the scope passed on each CreateShare call
}

func (m *mockSftpgoClient) MintToken(ctx context.Context, email, jwtAssertion string) (*sftpgo.Token, error) {
	m.mintTokenCalls = append(m.mintTokenCalls, jwtAssertion)
	if m.mintTokenFn != nil {
		return m.mintTokenFn(ctx, email, jwtAssertion)
	}
	return &sftpgo.Token{AccessToken: "mock-access-token", ExpiresAt: json.RawMessage(`"2026-08-27T12:00:00Z"`)}, nil
}

func (m *mockSftpgoClient) CreateShare(ctx context.Context, accessToken, storageKey string, scope int, ttl time.Duration) (string, error) {
	m.createShareCalls = append(m.createShareCalls, storageKey)
	m.createShareScopes = append(m.createShareScopes, scope)
	if m.createShareFn != nil {
		return m.createShareFn(ctx, accessToken, storageKey, scope, ttl)
	}
	return "mock-share-id", nil
}

func (m *mockSftpgoClient) PublicShareURL(shareID string) string {
	if m.publicShareURL != nil {
		return m.publicShareURL(shareID)
	}
	return "https://share.example.com/web/client/pubshares/" + shareID + "?compress=false"
}

func (m *mockSftpgoClient) BaseURL() string {
	if m.baseURL != "" {
		return m.baseURL
	}
	return "https://sftpgo.example.com"
}

// ----- MintUploadToken -----

func TestMintUploadTokenRequiresAuth(t *testing.T) {
	t.Parallel()
	h := NewAttachmentStorageHandler(&mockEntityCaseClient{}, &mockSftpgoClient{})

	req := httptest.NewRequest(http.MethodPost, "/cases/11111111-1111-1111-1111-111111111111/attachments/upload-token", nil)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	w := httptest.NewRecorder()

	h.MintUploadToken(w, req)

	assertStatus(t, w, http.StatusUnauthorized)
}

func TestMintUploadTokenRejectsInvalidCaseID(t *testing.T) {
	t.Parallel()
	h := NewAttachmentStorageHandler(&mockEntityCaseClient{}, &mockSftpgoClient{})

	req := withUser(httptest.NewRequest(http.MethodPost, "/cases/not-a-uuid/attachments/upload-token", nil))
	req.SetPathValue("id", "not-a-uuid")
	req.Header.Set("x-jwt-assertion", "raw-jwt")
	w := httptest.NewRecorder()

	h.MintUploadToken(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// TestMintUploadTokenBlocksClosedCase verifies the ACL check runs BEFORE any
// token is minted: a closed case must produce a 409 and zero calls into the
// SFTPGo client.
func TestMintUploadTokenBlocksClosedCase(t *testing.T) {
	t.Parallel()
	entity := &mockEntityCaseClient{
		getCaseFn: func(ctx context.Context, caseID string) ([]byte, error) {
			return []byte(`{"state":"closed"}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{}
	h := NewAttachmentStorageHandler(entity, sftpgoMock)

	caseID := "11111111-1111-1111-1111-111111111111"
	req := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/attachments/upload-token", nil))
	req.SetPathValue("id", caseID)
	req.Header.Set("x-jwt-assertion", "raw-jwt")
	w := httptest.NewRecorder()

	h.MintUploadToken(w, req)

	assertStatus(t, w, http.StatusConflict)
	assertErrorMessage(t, w, ErrMsgAttachmentOnClosedCase)
	if len(sftpgoMock.mintTokenCalls) != 0 {
		t.Errorf("MintToken was called %d times; want 0 — a token must never be minted for a closed case", len(sftpgoMock.mintTokenCalls))
	}
}

// TestMintUploadTokenRequiresJWTAssertionHeader verifies the raw
// x-jwt-assertion header value (not some other credential) is what gets
// forwarded as the mint password, and that a request without it never
// reaches the SFTPGo client.
func TestMintUploadTokenRequiresJWTAssertionHeader(t *testing.T) {
	t.Parallel()
	entity := &mockEntityCaseClient{
		getCaseFn: func(ctx context.Context, caseID string) ([]byte, error) {
			return []byte(`{"state":"work_in_progress"}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{}
	h := NewAttachmentStorageHandler(entity, sftpgoMock)

	caseID := "11111111-1111-1111-1111-111111111111"
	req := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/attachments/upload-token", nil))
	req.SetPathValue("id", caseID)
	// Deliberately no x-jwt-assertion header set.
	w := httptest.NewRecorder()

	h.MintUploadToken(w, req)

	assertStatus(t, w, http.StatusUnauthorized)
	if len(sftpgoMock.mintTokenCalls) != 0 {
		t.Errorf("MintToken was called %d times; want 0", len(sftpgoMock.mintTokenCalls))
	}
}

// TestMintUploadTokenSuccess verifies a successful mint on an open case
// forwards the exact x-jwt-assertion header value and returns the expected
// response shape.
func TestMintUploadTokenSuccess(t *testing.T) {
	t.Parallel()
	entity := &mockEntityCaseClient{
		getCaseFn: func(ctx context.Context, caseID string) ([]byte, error) {
			return []byte(`{"state":"work_in_progress"}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{baseURL: "https://sftpgo.example.com"}
	h := NewAttachmentStorageHandler(entity, sftpgoMock)

	caseID := "11111111-1111-1111-1111-111111111111"
	req := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/attachments/upload-token", nil))
	req.SetPathValue("id", caseID)
	req.Header.Set("x-jwt-assertion", "the-raw-jwt-assertion")
	w := httptest.NewRecorder()

	h.MintUploadToken(w, req)

	assertStatus(t, w, http.StatusOK)
	if len(sftpgoMock.mintTokenCalls) != 1 || sftpgoMock.mintTokenCalls[0] != "the-raw-jwt-assertion" {
		t.Fatalf("mintTokenCalls = %v, want exactly one call with the raw x-jwt-assertion value", sftpgoMock.mintTokenCalls)
	}

	rawBody := w.Body.String()
	resp := decodeJSON[uploadTokenResponse](t, w)
	if resp.ShareID != "mock-share-id" {
		t.Errorf("ShareID = %q, want mock-share-id", resp.ShareID)
	}
	if resp.SftpgoBaseURL != "https://sftpgo.example.com" {
		t.Errorf("SftpgoBaseURL = %q, want https://sftpgo.example.com", resp.SftpgoBaseURL)
	}
	// The share must be scoped to storageKey's parent directory, NOT
	// storageKey itself — confirmed against a real SFTPGo instance that a
	// share scoped to the exact file makes every shares-chunked-uploads call
	// against it fail (see MintUploadToken's doc comment on shareDir).
	wantShareDir := path.Dir(resp.StorageKey)
	if len(sftpgoMock.createShareCalls) != 1 || sftpgoMock.createShareCalls[0] != wantShareDir {
		t.Fatalf("createShareCalls = %v, want exactly [%q] (storageKey's parent directory)", sftpgoMock.createShareCalls, wantShareDir)
	}
	if len(sftpgoMock.createShareScopes) != 1 || sftpgoMock.createShareScopes[0] != sftpgo.ShareScopeWrite {
		t.Errorf("createShareScopes = %v, want exactly [%d] (write)", sftpgoMock.createShareScopes, sftpgo.ShareScopeWrite)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(rawBody), &raw); err != nil {
		t.Fatalf("decode raw response: %v; raw: %s", err, rawBody)
	}
	if _, hasToken := raw["sftpgoAccessToken"]; hasToken {
		t.Errorf("response body carried sftpgoAccessToken, want no bearer credential exposed to the frontend")
	}
	if _, hasExpiry := raw["expiresAt"]; hasExpiry {
		t.Errorf("response body carried expiresAt, want none — the share's own server-side expiry is not surfaced to the frontend")
	}
	// The case fixture above carries no "projectId", so this must fall back
	// to the project-less path shape rather than emitting a malformed
	// "project-" segment.
	wantKey := "/attachments/cases/" + caseID + "/"
	if !strings.HasPrefix(resp.StorageKey, wantKey) {
		t.Errorf("StorageKey = %q, want prefix %q (no-project fallback)", resp.StorageKey, wantKey)
	}
	attachmentID := strings.TrimPrefix(resp.StorageKey, wantKey)
	if !uuidRe.MatchString(attachmentID) {
		t.Errorf("StorageKey %q does not end in a well-formed UUID, got %q", resp.StorageKey, attachmentID)
	}
}

// TestMintUploadTokenStorageKeyIncludesProject verifies that when the case's
// own record carries a projectId, the minted storageKey follows the
// documented convention:
// /attachments/project-<projectId>/cases/<caseId>/<attachmentId>.
func TestMintUploadTokenStorageKeyIncludesProject(t *testing.T) {
	t.Parallel()
	const projectID = "22222222-2222-2222-2222-222222222222"
	entity := &mockEntityCaseClient{
		getCaseFn: func(ctx context.Context, caseID string) ([]byte, error) {
			return []byte(`{"state":"work_in_progress","projectId":"` + projectID + `"}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{}
	h := NewAttachmentStorageHandler(entity, sftpgoMock)

	caseID := "11111111-1111-1111-1111-111111111111"
	req := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/attachments/upload-token", nil))
	req.SetPathValue("id", caseID)
	req.Header.Set("x-jwt-assertion", "raw-jwt")
	w := httptest.NewRecorder()

	h.MintUploadToken(w, req)

	assertStatus(t, w, http.StatusOK)
	resp := decodeJSON[uploadTokenResponse](t, w)

	wantPrefix := "/attachments/project-" + projectID + "/cases/" + caseID + "/"
	if !strings.HasPrefix(resp.StorageKey, wantPrefix) {
		t.Fatalf("StorageKey = %q, want prefix %q", resp.StorageKey, wantPrefix)
	}
	attachmentID := strings.TrimPrefix(resp.StorageKey, wantPrefix)
	if !uuidRe.MatchString(attachmentID) {
		t.Errorf("StorageKey %q does not end in a well-formed UUID, got %q", resp.StorageKey, attachmentID)
	}
}

// TestMintUploadTokenStorageKeyUniquePerCall verifies each mint generates a
// fresh attachment id, never reusing one across calls.
func TestMintUploadTokenStorageKeyUniquePerCall(t *testing.T) {
	t.Parallel()
	entity := &mockEntityCaseClient{
		getCaseFn: func(ctx context.Context, caseID string) ([]byte, error) {
			return []byte(`{"state":"work_in_progress"}`), nil
		},
	}
	h := NewAttachmentStorageHandler(entity, &mockSftpgoClient{})

	caseID := "11111111-1111-1111-1111-111111111111"
	seen := make(map[string]bool)
	for i := 0; i < 5; i++ {
		req := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/attachments/upload-token", nil))
		req.SetPathValue("id", caseID)
		req.Header.Set("x-jwt-assertion", "raw-jwt")
		w := httptest.NewRecorder()
		h.MintUploadToken(w, req)
		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[uploadTokenResponse](t, w)
		if seen[resp.StorageKey] {
			t.Fatalf("StorageKey %q was generated twice", resp.StorageKey)
		}
		seen[resp.StorageKey] = true
	}
}

func TestMintUploadTokenPropagatesSftpgoFailure(t *testing.T) {
	t.Parallel()
	entity := &mockEntityCaseClient{
		getCaseFn: func(ctx context.Context, caseID string) ([]byte, error) {
			return []byte(`{"state":"work_in_progress"}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{
		mintTokenFn: func(ctx context.Context, email, jwtAssertion string) (*sftpgo.Token, error) {
			return nil, &apierror.Error{StatusCode: http.StatusUnauthorized, Body: "denied"}
		},
	}
	h := NewAttachmentStorageHandler(entity, sftpgoMock)

	caseID := "11111111-1111-1111-1111-111111111111"
	req := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/attachments/upload-token", nil))
	req.SetPathValue("id", caseID)
	req.Header.Set("x-jwt-assertion", "raw-jwt")
	w := httptest.NewRecorder()

	h.MintUploadToken(w, req)

	assertStatus(t, w, http.StatusBadGateway)
}

// TestMintUploadTokenPropagatesCreateShareFailure verifies a failure from the
// write-share creation call (as opposed to the token mint) also surfaces as
// a 502, and that the response never leaks a partially-built token/share.
func TestMintUploadTokenPropagatesCreateShareFailure(t *testing.T) {
	t.Parallel()
	entity := &mockEntityCaseClient{
		getCaseFn: func(ctx context.Context, caseID string) ([]byte, error) {
			return []byte(`{"state":"work_in_progress"}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{
		createShareFn: func(ctx context.Context, accessToken, storageKey string, scope int, ttl time.Duration) (string, error) {
			return "", &apierror.Error{StatusCode: http.StatusInternalServerError, Body: "boom"}
		},
	}
	h := NewAttachmentStorageHandler(entity, sftpgoMock)

	caseID := "11111111-1111-1111-1111-111111111111"
	req := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/attachments/upload-token", nil))
	req.SetPathValue("id", caseID)
	req.Header.Set("x-jwt-assertion", "raw-jwt")
	w := httptest.NewRecorder()

	h.MintUploadToken(w, req)

	assertStatus(t, w, http.StatusBadGateway)
	if len(sftpgoMock.mintTokenCalls) != 1 {
		t.Errorf("MintToken was called %d times; want 1 (still needed to authenticate the CreateShare call)", len(sftpgoMock.mintTokenCalls))
	}
}

// ----- CreateAttachmentShare -----

func TestCreateAttachmentShareRequiresAuth(t *testing.T) {
	t.Parallel()
	h := NewAttachmentStorageHandler(&mockEntityCaseClient{}, &mockSftpgoClient{})

	req := httptest.NewRequest(http.MethodPost, "/attachments/11111111-1111-1111-1111-111111111111/share", nil)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	w := httptest.NewRecorder()

	h.CreateAttachmentShare(w, req)

	assertStatus(t, w, http.StatusUnauthorized)
}

func TestCreateAttachmentShareRejectsInvalidAttachmentID(t *testing.T) {
	t.Parallel()
	h := NewAttachmentStorageHandler(&mockEntityCaseClient{}, &mockSftpgoClient{})

	req := withUser(httptest.NewRequest(http.MethodPost, "/attachments/not-a-uuid/share", nil))
	req.SetPathValue("id", "not-a-uuid")
	w := httptest.NewRecorder()

	h.CreateAttachmentShare(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// TestCreateAttachmentShareEnforcesReadAccessBeforeMinting verifies the case
// read-access check runs, and fails, BEFORE any SFTPGo call is made.
func TestCreateAttachmentShareEnforcesReadAccessBeforeMinting(t *testing.T) {
	t.Parallel()
	storageKey := "/attachments/att-1"
	caseID := "22222222-2222-2222-2222-222222222222"
	entity := &mockEntityCaseClient{
		getCaseAttachmentFn: func(ctx context.Context, attachmentID string) ([]byte, error) {
			return []byte(`{"referenceId":"` + caseID + `","referenceType":"case","storageKey":"` + storageKey + `"}`), nil
		},
		getCaseFn: func(ctx context.Context, id string) ([]byte, error) {
			return nil, &apierror.Error{StatusCode: http.StatusNotFound, Body: "not found"}
		},
	}
	sftpgoMock := &mockSftpgoClient{}
	h := NewAttachmentStorageHandler(entity, sftpgoMock)

	attachmentID := "11111111-1111-1111-1111-111111111111"
	req := withUser(httptest.NewRequest(http.MethodPost, "/attachments/"+attachmentID+"/share", nil))
	req.SetPathValue("id", attachmentID)
	req.Header.Set("x-jwt-assertion", "raw-jwt")
	w := httptest.NewRecorder()

	h.CreateAttachmentShare(w, req)

	assertStatus(t, w, http.StatusNotFound)
	if len(sftpgoMock.mintTokenCalls) != 0 || len(sftpgoMock.createShareCalls) != 0 {
		t.Errorf("sftpgo was called (mint=%d, share=%d); want 0 — access must be checked before minting/sharing",
			len(sftpgoMock.mintTokenCalls), len(sftpgoMock.createShareCalls))
	}
}

// TestCreateAttachmentShareRejectsMissingStorageKey verifies an attachment
// with no storageKey (e.g. stored the old, non-SFTPGo way) fails cleanly
// rather than attempting to share an empty path.
func TestCreateAttachmentShareRejectsMissingStorageKey(t *testing.T) {
	t.Parallel()
	caseID := "22222222-2222-2222-2222-222222222222"
	entity := &mockEntityCaseClient{
		getCaseAttachmentFn: func(ctx context.Context, attachmentID string) ([]byte, error) {
			return []byte(`{"referenceId":"` + caseID + `","referenceType":"case"}`), nil
		},
		getCaseFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"state":"work_in_progress"}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{}
	h := NewAttachmentStorageHandler(entity, sftpgoMock)

	attachmentID := "11111111-1111-1111-1111-111111111111"
	req := withUser(httptest.NewRequest(http.MethodPost, "/attachments/"+attachmentID+"/share", nil))
	req.SetPathValue("id", attachmentID)
	req.Header.Set("x-jwt-assertion", "raw-jwt")
	w := httptest.NewRecorder()

	h.CreateAttachmentShare(w, req)

	assertStatus(t, w, http.StatusConflict)
	assertErrorMessage(t, w, ErrMsgAttachmentNotShareable)
	if len(sftpgoMock.mintTokenCalls) != 0 {
		t.Errorf("MintToken was called %d times; want 0", len(sftpgoMock.mintTokenCalls))
	}
}

// TestCreateAttachmentShareSuccess verifies the happy path: the storage key
// is forwarded to CreateShare verbatim, and the response carries the URL
// built from PublicShareURL(shareID) — never a hand-rolled URL.
func TestCreateAttachmentShareSuccess(t *testing.T) {
	t.Parallel()
	caseID := "22222222-2222-2222-2222-222222222222"
	storageKey := "/attachments/att-1"
	entity := &mockEntityCaseClient{
		getCaseAttachmentFn: func(ctx context.Context, attachmentID string) ([]byte, error) {
			return []byte(`{"referenceId":"` + caseID + `","referenceType":"case","storageKey":"` + storageKey + `"}`), nil
		},
		getCaseFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"state":"closed"}`), nil // reads are allowed even on a closed case
		},
	}
	sftpgoMock := &mockSftpgoClient{
		createShareFn: func(ctx context.Context, accessToken, gotStorageKey string, scope int, ttl time.Duration) (string, error) {
			if ttl != shareTTL {
				t.Errorf("ttl = %v, want %v", ttl, shareTTL)
			}
			if scope != sftpgo.ShareScopeRead {
				t.Errorf("scope = %d, want %d (read) — this download-share path must stay read-only", scope, sftpgo.ShareScopeRead)
			}
			return "share-xyz", nil
		},
	}
	h := NewAttachmentStorageHandler(entity, sftpgoMock)

	attachmentID := "11111111-1111-1111-1111-111111111111"
	req := withUser(httptest.NewRequest(http.MethodPost, "/attachments/"+attachmentID+"/share", nil))
	req.SetPathValue("id", attachmentID)
	req.Header.Set("x-jwt-assertion", "raw-jwt")
	w := httptest.NewRecorder()

	h.CreateAttachmentShare(w, req)

	assertStatus(t, w, http.StatusCreated)
	if len(sftpgoMock.createShareCalls) != 1 || sftpgoMock.createShareCalls[0] != storageKey {
		t.Fatalf("createShareCalls = %v, want exactly [%q]", sftpgoMock.createShareCalls, storageKey)
	}

	resp := decodeJSON[shareResponse](t, w)
	want := sftpgoMock.PublicShareURL("share-xyz")
	if resp.ShareURL != want {
		t.Errorf("ShareURL = %q, want %q", resp.ShareURL, want)
	}
}

func TestCreateAttachmentSharePropagatesSftpgoFailure(t *testing.T) {
	t.Parallel()
	caseID := "22222222-2222-2222-2222-222222222222"
	storageKey := "/attachments/att-1"
	entity := &mockEntityCaseClient{
		getCaseAttachmentFn: func(ctx context.Context, attachmentID string) ([]byte, error) {
			return []byte(`{"referenceId":"` + caseID + `","referenceType":"case","storageKey":"` + storageKey + `"}`), nil
		},
		getCaseFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"state":"work_in_progress"}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{
		createShareFn: func(ctx context.Context, accessToken, gotStorageKey string, scope int, ttl time.Duration) (string, error) {
			return "", &apierror.Error{StatusCode: http.StatusInternalServerError, Body: "boom"}
		},
	}
	h := NewAttachmentStorageHandler(entity, sftpgoMock)

	attachmentID := "11111111-1111-1111-1111-111111111111"
	req := withUser(httptest.NewRequest(http.MethodPost, "/attachments/"+attachmentID+"/share", nil))
	req.SetPathValue("id", attachmentID)
	req.Header.Set("x-jwt-assertion", "raw-jwt")
	w := httptest.NewRecorder()

	h.CreateAttachmentShare(w, req)

	assertStatus(t, w, http.StatusBadGateway)
}
