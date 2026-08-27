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

// Package sftpgo is an HTTP client for the small subset of SFTPGo's REST API
// this backend calls when the SFTPGo-backed attachment-storage feature flag
// (SFTPGO_ATTACHMENT_STORAGE_ENABLED) is on: minting a short-lived per-user
// access token, and creating a short-lived public download share. This
// package never touches attachment bytes: uploads and downloads always go
// directly between the browser and SFTPGo using the credentials it mints
// here — see internal/handler.AttachmentStorageHandler for the call sites.
package sftpgo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// maxErrBodyBytes bounds how much of a non-2xx response body is retained on
// an *apierror.Error, mirroring internal/entity's CustomerEntityClient.
const maxErrBodyBytes = 256

// Config holds the configuration for a SFTPGo Client.
type Config struct {
	// BaseURL is SFTPGo's REST API base (used for both the token-mint and
	// share-creation calls below), and is also the host the FE is told to
	// call directly for the chunked/TUS upload once it has a minted token —
	// SFTPGo serves both its REST API and its upload endpoints from the same
	// httpd listener, so one base URL covers both.
	BaseURL string
	// PublicBaseURL is the host used to construct the public share URL
	// returned by CreateShare's caller (see PublicShareURL). SFTPGo commonly
	// fronts its WebClient (share pages) on a different public host/port
	// than its REST API, so this is a separate, optional value; when unset
	// it defaults to BaseURL.
	PublicBaseURL string
}

// Client is a minimal SFTPGo REST API client scoped to token-mint and
// share-creation — the only two calls this backend ever makes to SFTPGo.
type Client struct {
	http          *http.Client
	baseURL       string
	publicBaseURL string
}

// NewClient constructs a SFTPGo Client from cfg.
func NewClient(cfg Config) *Client {
	publicBaseURL := cfg.PublicBaseURL
	if publicBaseURL == "" {
		publicBaseURL = cfg.BaseURL
	}
	return &Client{
		http:          &http.Client{Timeout: 15 * time.Second},
		baseURL:       strings.TrimRight(cfg.BaseURL, "/"),
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}
}

// BaseURL returns the configured REST API base URL, verbatim — handed back
// to the FE by AttachmentStorageHandler.MintUploadToken as the host it
// should call directly for the upload itself.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Token is the response body of SFTPGo's GET /api/v2/user/token. ExpiresAt is
// kept as raw JSON and passed through unmodified rather than parsed into a
// Go time value, since its exact wire type (string vs. epoch number) was not
// verified against a live instance for this change.
type Token struct {
	AccessToken string          `json:"access_token"`
	ExpiresAt   json.RawMessage `json:"expires_at"`
}

// MintToken calls SFTPGo's GET /api/v2/user/token using HTTP Basic auth:
// username is the caller's email claim, password is the caller's raw
// gateway-issued JWT (the x-jwt-assertion header value this backend itself
// already validated). SFTPGo's external_auth_hook (see
// operations/sftpgo-authentication-service's /external-auth-hook)
// independently re-validates that JWT against the same JWKS/issuer/audience
// this backend trusts, so the "password" here is never a SFTPGo-native
// credential — it is the same bearer token the caller already presented to
// this backend.
func (c *Client) MintToken(ctx context.Context, email, jwtAssertion string) (*Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/user/token", nil)
	if err != nil {
		return nil, fmt.Errorf("sftpgo: build token request: %w", err)
	}
	req.SetBasicAuth(email, jwtAssertion)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sftpgo: token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("sftpgo: read token response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: truncate(body)}
	}

	var tok Token
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("sftpgo: decode token response: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("sftpgo: token response carried no access_token")
	}
	return &tok, nil
}

// shareScopeRead is SFTPGo's Share.Scope value for a read-only (download)
// share. Not verified against a live instance for this change — flagged
// explicitly in the PR description alongside the other SFTPGo API-shape
// assumptions this client makes.
const shareScopeRead = 1

// shareCreateRequest is the request body of POST /api/v2/user/shares.
type shareCreateRequest struct {
	Paths     []string `json:"paths"`
	Scope     int      `json:"scope"`
	ExpiresAt int64    `json:"expires_at"`
}

// shareCreateResponseBody is the fallback shape checked when the share id is
// not present on the X-Object-Id response header (see CreateShare). Both
// field names are checked since which one (if either) SFTPGo actually uses
// here was not verified against a live instance.
type shareCreateResponseBody struct {
	ID      string `json:"id"`
	ShareID string `json:"share_id"`
}

// CreateShare calls SFTPGo's POST /api/v2/user/shares, authenticated as the
// caller via accessToken (minted by MintToken), to create a short-lived,
// read-only public share for a single storage path. ttl controls how soon
// the share expires; callers should keep this short since a share is created
// fresh on every request that needs one (see
// AttachmentStorageHandler.CreateAttachmentShare — this is a lazy,
// per-attachment, per-request operation, never an eager batch one). Returns
// the created share's id.
func (c *Client) CreateShare(ctx context.Context, accessToken, storageKey string, ttl time.Duration) (string, error) {
	reqBody, err := json.Marshal(shareCreateRequest{
		Paths:     []string{storageKey},
		Scope:     shareScopeRead,
		ExpiresAt: time.Now().Add(ttl).UnixMilli(),
	})
	if err != nil {
		return "", fmt.Errorf("sftpgo: encode share request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/user/shares", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("sftpgo: build share request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("sftpgo: share request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("sftpgo: read share response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", &apierror.Error{StatusCode: resp.StatusCode, Body: truncate(body)}
	}

	// SFTPGo has historically returned the created object's id via the
	// X-Object-Id response header rather than the JSON body — confirmed
	// empirically against a real instance in a prior session. The JSON body
	// is only a fallback here and has NOT been independently re-verified for
	// this change.
	if id := resp.Header.Get("X-Object-Id"); id != "" {
		return id, nil
	}

	var decoded shareCreateResponseBody
	if err := json.Unmarshal(body, &decoded); err == nil {
		if decoded.ID != "" {
			return decoded.ID, nil
		}
		if decoded.ShareID != "" {
			return decoded.ShareID, nil
		}
	}

	return "", fmt.Errorf("sftpgo: share response carried no id (checked X-Object-Id header and id/share_id body fields)")
}

// PublicShareURL builds the public download URL for a share id.
//
// This is deliberately NOT "{publicBaseURL}/shares/{id}", despite that being
// what SFTPGo's own OpenAPI path naming might suggest — the working path,
// confirmed empirically against a real instance in a prior session, is
// "/web/client/pubshares/{id}".
func (c *Client) PublicShareURL(shareID string) string {
	return c.publicBaseURL + "/web/client/pubshares/" + url.PathEscape(shareID) + "?compress=false"
}

// truncate bounds body to maxErrBodyBytes for inclusion on an *apierror.Error.
func truncate(body []byte) string {
	if len(body) > maxErrBodyBytes {
		body = body[:maxErrBodyBytes]
	}
	return string(body)
}
