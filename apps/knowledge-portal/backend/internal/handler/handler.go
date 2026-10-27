// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.

// Package handler serves the public Knowledge Base API. It talks to
// entity-service with a plain HTTP client (no OAuth, no user token) and
// only ever exposes published articles -- drafts and in-review articles
// never leave the CSM Portal.
package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"time"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isValidUUID(s string) bool { return uuidRe.MatchString(s) }

// publicArticle is the subset of an entity-service article that is safe to
// expose publicly. Internal fields (authorId, revisedById, updatedBy,
// sourceCaseId, rejectionComment, baseVersionId, latest) are stripped.
type publicArticle struct {
	ID              string  `json:"id"`
	KnowledgeBaseID string  `json:"knowledgeBaseId"`
	Title           string  `json:"title"`
	Body            string  `json:"body"`
	Number          string  `json:"number,omitempty"`
	State           string  `json:"state"`
	ViewCount       int     `json:"viewCount"`
	Rating          *float64 `json:"rating,omitempty"`
	CreatedOn       string  `json:"createdOn,omitempty"`
	UpdatedOn       string  `json:"updatedOn,omitempty"`
	PublishedOn     string  `json:"publishedOn,omitempty"`
	RetiredOn       string  `json:"retiredOn,omitempty"`
}

type KBHandler struct {
	http         *http.Client
	baseURL      string
	pineconeHost string
	pineconeKey  string
}

func NewKBHandler() *KBHandler {
	baseURL := os.Getenv("ENTITY_SERVICE_BASE_URL")
	if baseURL == "" {
		slog.Error("ENTITY_SERVICE_BASE_URL is not set; exiting")
		os.Exit(1)
	}
	return &KBHandler{
		http: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		baseURL:      baseURL,
		pineconeHost: os.Getenv("PINECONE_HOST"),
		pineconeKey:  os.Getenv("PINECONE_KEY"),
	}
}

type searchRequest struct {
	KnowledgeBaseID string `json:"knowledgeBaseId,omitempty"`
	SearchQuery     string `json:"searchQuery,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	Offset          int    `json:"offset,omitempty"`
}

// ListKnowledgeBases handles GET /public/knowledge-bases.
// Only active knowledge bases are returned.
func (h *KBHandler) ListKnowledgeBases(w http.ResponseWriter, r *http.Request) {
	resp, err := h.do(r, http.MethodGet, "/knowledge-bases", nil)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity-service call failed", "path", "/knowledge-bases", "err", err)
		writeError(w, http.StatusBadGateway, "The knowledge base is temporarily unavailable.")
		return
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		writeError(w, http.StatusBadGateway, "Failed to read the response.")
		return
	}
	if resp.StatusCode != http.StatusOK {
		slog.ErrorContext(r.Context(), "entity-service returned non-200", "path", r.URL.Path, "status", resp.StatusCode)
		writeError(w, http.StatusBadGateway, "The knowledge base is temporarily unavailable.")
		return
	}

	// Filter to active-only before sending to the public client.
	var upstream struct {
		KnowledgeBases []json.RawMessage `json:"knowledgeBases"`
	}
	if err := json.Unmarshal(payload, &upstream); err != nil {
		slog.ErrorContext(r.Context(), "decode knowledge bases failed", "err", err)
		writeError(w, http.StatusBadGateway, "The knowledge base is temporarily unavailable.")
		return
	}

	active := make([]json.RawMessage, 0, len(upstream.KnowledgeBases))
	for _, raw := range upstream.KnowledgeBases {
		var kb struct {
			Active bool `json:"active"`
		}
		if json.Unmarshal(raw, &kb) == nil && kb.Active {
			active = append(active, raw)
		}
	}
	out, _ := json.Marshal(map[string]any{"knowledgeBases": active})
	writeJSON(w, http.StatusOK, out)
}

// SearchArticles handles POST /public/kb-articles/search. The state filter
// is forced to published server-side and never taken from the request.
// sortBy is intentionally not forwarded — upstream does not support it.
func (h *KBHandler) SearchArticles(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req)
	}

	limit := req.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	upstream := map[string]any{
		"states":     []string{"published"},
		"pagination": map[string]int{"limit": limit, "offset": offset},
	}
	if req.KnowledgeBaseID != "" {
		upstream["knowledgeBaseId"] = req.KnowledgeBaseID
	}
	if req.SearchQuery != "" {
		upstream["searchQuery"] = req.SearchQuery
	}

	body, err := json.Marshal(upstream)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to build the search request.")
		return
	}

	resp, err := h.do(r, http.MethodPost, "/kb-articles/search", body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity-service call failed", "path", "/kb-articles/search", "err", err)
		writeError(w, http.StatusBadGateway, "The knowledge base is temporarily unavailable.")
		return
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		writeError(w, http.StatusBadGateway, "Failed to read the response.")
		return
	}
	if resp.StatusCode != http.StatusOK {
		slog.ErrorContext(r.Context(), "entity-service returned non-200", "path", r.URL.Path, "status", resp.StatusCode)
		writeError(w, http.StatusBadGateway, "The knowledge base is temporarily unavailable.")
		return
	}

	// Strip internal fields from each article before sending to the public.
	var searchResp struct {
		Articles []json.RawMessage `json:"articles"`
		Total    int               `json:"total"`
	}
	if err := json.Unmarshal(payload, &searchResp); err != nil {
		slog.ErrorContext(r.Context(), "decode search response failed", "err", err)
		writeError(w, http.StatusBadGateway, "The knowledge base is temporarily unavailable.")
		return
	}

	safe := make([]publicArticle, 0, len(searchResp.Articles))
	for _, raw := range searchResp.Articles {
		var a publicArticle
		if json.Unmarshal(raw, &a) == nil {
			safe = append(safe, a)
		}
	}
	out, _ := json.Marshal(map[string]any{"articles": safe, "total": searchResp.Total})
	writeJSON(w, http.StatusOK, out)
}

// GetArticle handles GET /public/kb-articles/{id}. An unpublished article
// is reported as 404, not 403, so a caller can't detect that a draft with
// that id exists. Only safe public fields are returned.
func (h *KBHandler) GetArticle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidUUID(id) {
		writeError(w, http.StatusBadRequest, "Invalid article id.")
		return
	}

	resp, err := h.do(r, http.MethodGet, "/kb-articles/"+id, nil)
	if err != nil {
		slog.ErrorContext(r.Context(), "get article failed", "id", id, "err", err)
		writeError(w, http.StatusBadGateway, "Failed to retrieve the article.")
		return
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		writeError(w, http.StatusBadGateway, "Failed to read the article.")
		return
	}
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusNotFound, "Article not found.")
		return
	}

	var full struct {
		Article json.RawMessage `json:"article"`
	}
	if err := json.Unmarshal(payload, &full); err != nil {
		var a publicArticle
		if json.Unmarshal(payload, &a) == nil && a.State == "published" {
			out, _ := json.Marshal(map[string]any{"article": a})
			writeJSON(w, http.StatusOK, out)
			return
		}
		writeError(w, http.StatusBadGateway, "Failed to read the article.")
		return
	}

	var a publicArticle
	if err := json.Unmarshal(full.Article, &a); err != nil {
		writeError(w, http.StatusBadGateway, "Failed to read the article.")
		return
	}
	if a.State != "published" {
		writeError(w, http.StatusNotFound, "Article not found.")
		return
	}

	out, _ := json.Marshal(map[string]any{"article": a})
	writeJSON(w, http.StatusOK, out)
}

func (h *KBHandler) do(r *http.Request, method, path string, body []byte) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(r.Context(), method, h.baseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return h.http.Do(req)
}

func writeJSON(w http.ResponseWriter, status int, payload []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// CORS allows the webapp (a different origin in dev) to call this API.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
