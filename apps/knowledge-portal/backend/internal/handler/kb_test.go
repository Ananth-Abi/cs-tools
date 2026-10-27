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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/pinecone"
)

const (
	uuidA = "11111111-1111-1111-1111-111111111111"
	uuidB = "22222222-2222-2222-2222-222222222222"
	uuidC = "33333333-3333-3333-3333-333333333333"
)

// fakeEntity records the calls it receives and serves canned answers.
type fakeEntity struct {
	mu        sync.Mutex
	kbs       []entity.KnowledgeBase
	kbsErr    error
	articles  map[string]entity.KBArticle
	getErr    map[string]error
	search    entity.SearchResponse
	searchErr error

	getIDs      []string
	kbCalls     int
	searchCalls []entity.SearchRequest
}

func (f *fakeEntity) ListKnowledgeBases(context.Context) ([]entity.KnowledgeBase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.kbCalls++
	if f.kbs == nil && f.kbsErr == nil {
		// Default: the one knowledge base every test article lives in is active.
		return []entity.KnowledgeBase{{ID: "kb-1", Title: "One", Active: true}}, nil
	}
	return f.kbs, f.kbsErr
}

func (f *fakeEntity) SearchKBArticles(_ context.Context, req entity.SearchRequest) (entity.SearchResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.searchCalls = append(f.searchCalls, req)
	return f.search, f.searchErr
}

func (f *fakeEntity) GetKBArticle(_ context.Context, id string) (entity.KBArticle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getIDs = append(f.getIDs, id)
	if err := f.getErr[id]; err != nil {
		return entity.KBArticle{}, err
	}
	a, ok := f.articles[id]
	if !ok {
		return entity.KBArticle{}, &apierror.Error{StatusCode: http.StatusNotFound, Body: "nope"}
	}
	return a, nil
}

type fakeSimilarity struct {
	matches []pinecone.Match
	err     error
}

func (f fakeSimilarity) Similar(context.Context, string, int) ([]pinecone.Match, error) {
	return f.matches, f.err
}

func article(id, state string) entity.KBArticle {
	pub := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return entity.KBArticle{
		ID: id, KnowledgeBaseID: "kb-1", Title: "Title " + id, Body: "<p>body</p>", State: state,
		CreatedOn: pub, UpdatedOn: pub, PublishedOn: &pub,
	}
}

// newMux wires the handlers on the same patterns main.go registers.
func newMux(e Entity, s Similarity) *http.ServeMux {
	kb := NewKBHandler(e, s)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /public/knowledge-bases", kb.ListKnowledgeBases)
	mux.HandleFunc("POST /public/kb-articles/search", kb.SearchArticles)
	mux.HandleFunc("GET /public/kb-articles/{id}", kb.GetArticle)
	mux.HandleFunc("GET /public/kb-articles/{id}/related", kb.RelatedArticles)
	return mux
}

func do(mux http.Handler, method, target, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestValidID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{uuidA, true},
		{"abc_DEF-123", true},
		{strings.Repeat("a", 64), true},
		{strings.Repeat("a", 65), false},
		{"", false},
		{"..", false},
		{"a/b", false},
		{"a%2Fb", false},
		{"..%2F..%2Fcases", false},
		{"a?x=1", false},
		{"a#frag", false},
		{"a b", false},
		{"a\nb", false},
		{"idé", false},
	}
	for _, tt := range tests {
		if got := validID(tt.id); got != tt.want {
			t.Errorf("validID(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestGetArticleRejectsBadIDsBeforeUpstream(t *testing.T) {
	for _, id := range []string{"a%2F..%2Fcases", "a%3Fx%3D1", "a%23b", "a%2e%2e", strings.Repeat("a", 65)} {
		t.Run(id, func(t *testing.T) {
			f := &fakeEntity{}
			rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/"+id, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body)
			}
			if len(f.getIDs) != 0 {
				t.Fatalf("upstream was called with %v for a rejected id", f.getIDs)
			}
		})
	}
}

func TestGetArticlePublishedOnlyAndDTOWhitelist(t *testing.T) {
	leaky := article(uuidA, "published")
	leaky.Number = ptr("KB0001")
	tests := []struct {
		name       string
		article    entity.KBArticle
		upstream   error
		wantStatus int
	}{
		{"published", leaky, nil, http.StatusOK},
		{"draft is 404", article(uuidA, "draft"), nil, http.StatusNotFound},
		{"pending_review is 404", article(uuidA, "pending_review"), nil, http.StatusNotFound},
		{"retired is 404", article(uuidA, "retired"), nil, http.StatusNotFound},
		{"empty state is 404", article(uuidA, ""), nil, http.StatusNotFound},
		{"upstream 404", entity.KBArticle{}, &apierror.Error{StatusCode: 404}, http.StatusNotFound},
		{"upstream 403 is 502", entity.KBArticle{}, &apierror.Error{StatusCode: 403, Body: "secret detail"}, http.StatusBadGateway},
		{"upstream 500 is 502", entity.KBArticle{}, &apierror.Error{StatusCode: 500, Body: "stack trace"}, http.StatusBadGateway},
		{"upstream timeout is 504", entity.KBArticle{}, context.DeadlineExceeded, http.StatusGatewayTimeout},
		{"transport error is 502", entity.KBArticle{}, errors.New("dial tcp 10.0.0.1: refused"), http.StatusBadGateway},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeEntity{articles: map[string]entity.KBArticle{uuidA: tt.article}, getErr: map[string]error{}}
			if tt.upstream != nil {
				f.getErr[uuidA] = tt.upstream
			}
			rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/"+uuidA, "")
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			body := rec.Body.String()
			for _, leak := range []string{"secret detail", "stack trace", "10.0.0.1", "refused"} {
				if strings.Contains(body, leak) {
					t.Errorf("response leaks upstream detail %q: %s", leak, body)
				}
			}
			if tt.wantStatus != http.StatusOK {
				var e errorBody
				if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Message == "" {
					t.Errorf("want {message} envelope, got %s", body)
				}
				return
			}
			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			want := map[string]bool{"id": true, "knowledgeBaseId": true, "title": true, "body": true, "number": true,
				"createdOn": true, "updatedOn": true, "publishedOn": true}
			for k := range got {
				if !want[k] {
					t.Errorf("unexpected field %q in public article", k)
				}
			}
			for k := range want {
				if _, ok := got[k]; !ok {
					t.Errorf("missing field %q", k)
				}
			}
		})
	}
}

// TestArticleDTODropsInternalFields feeds the real entity-service JSON shape
// (with every internal field populated) through the client types and checks the
// served JSON never mentions them.
func TestArticleDTODropsInternalFields(t *testing.T) {
	const upstream = `{"id":"` + uuidA + `","knowledgeBaseId":"kb-1","title":"T","body":"<p>b</p>","state":"published",
		"authorId":"author-secret","revisedById":"reviser-secret","sourceCaseId":"case-secret","rejectionComment":"reject-secret",
		"updatedBy":"updater-secret","baseVersionId":"base-secret","latest":true,"createdOn":"2026-01-01T00:00:00Z",
		"updatedOn":"2026-01-02T00:00:00Z","publishedOn":"2026-01-03T00:00:00Z","retiredOn":null}`
	var a entity.KBArticle
	if err := json.Unmarshal([]byte(upstream), &a); err != nil {
		t.Fatal(err)
	}
	f := &fakeEntity{articles: map[string]entity.KBArticle{uuidA: a}}
	rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/"+uuidA, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	for _, leak := range []string{"secret", "authorId", "revisedById", "sourceCaseId", "rejectionComment", "updatedBy", "baseVersionId", "latest", "state"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("public article contains %q: %s", leak, rec.Body)
		}
	}
}

func TestSearchForcesPublishedAndFiltersResults(t *testing.T) {
	f := &fakeEntity{search: entity.SearchResponse{
		Articles: []entity.KBArticle{article(uuidA, "published"), article(uuidB, "draft"), article(uuidC, "retired")},
		Total:    3, Limit: 20, Offset: 0,
	}}
	// "states" in the body must be rejected, not honoured.
	rec := do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", `{"states":["draft"]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("states in body: status = %d, want 400", rec.Code)
	}
	if len(f.searchCalls) != 0 {
		t.Fatal("upstream called for a rejected request")
	}

	rec = do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", `{"knowledgeBaseId":"kb-1","searchQuery":"  oauth  ","limit":10,"offset":5}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if len(f.searchCalls) != 1 {
		t.Fatalf("search calls = %d", len(f.searchCalls))
	}
	got := f.searchCalls[0]
	if len(got.States) != 1 || got.States[0] != "published" {
		t.Errorf("States = %v, want [published]", got.States)
	}
	if got.KnowledgeBaseID != "kb-1" || got.SearchQuery != "oauth" || got.Pagination != (entity.Pagination{Limit: 10, Offset: 5}) {
		t.Errorf("upstream request = %+v", got)
	}
	var resp struct {
		Articles []map[string]any `json:"articles"`
		Total    int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Articles) != 1 || resp.Articles[0]["id"] != uuidA {
		t.Errorf("non-published articles leaked through: %s", rec.Body)
	}
}

func TestSearchUpstreamRequestHasNoSortBy(t *testing.T) {
	// entity-service decodes strictly: a sortBy key would be a 400 upstream.
	b, _ := json.Marshal(entity.SearchRequest{States: []string{"published"}})
	if strings.Contains(string(b), "sortBy") {
		t.Fatalf("upstream search body carries sortBy: %s", b)
	}
}

func TestSearchInputBounds(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"empty body", ``, 200},
		{"empty object", `{}`, 200},
		{"limit default", `{"limit":0}`, 200},
		{"limit max", `{"limit":50}`, 200},
		{"limit over", `{"limit":51}`, 400},
		{"limit negative", `{"limit":-1}`, 400},
		{"offset max", `{"offset":10000}`, 200},
		{"offset over", `{"offset":10001}`, 400},
		{"offset negative", `{"offset":-1}`, 400},
		{"query at cap", `{"searchQuery":"` + strings.Repeat("a", 200) + `"}`, 200},
		{"query over cap", `{"searchQuery":"` + strings.Repeat("a", 201) + `"}`, 400},
		{"query multibyte at cap", `{"searchQuery":"` + strings.Repeat("é", 200) + `"}`, 200},
		{"query control char", `{"searchQuery":"a\u0000b"}`, 400},
		{"kb id ok", `{"knowledgeBaseId":"kb-1"}`, 200},
		{"kb id valid but unknown", `{"knowledgeBaseId":"` + uuidA + `"}`, 404},
		{"kb id traversal", `{"knowledgeBaseId":"../x"}`, 400},
		{"kb id too long", `{"knowledgeBaseId":"` + strings.Repeat("a", 65) + `"}`, 400},
		{"sortBy recent", `{"sortBy":"recent"}`, 200},
		{"sortBy mostViewed unsupported", `{"sortBy":"mostViewed"}`, 400},
		{"sortBy topRated unsupported", `{"sortBy":"topRated"}`, 400},
		{"sortBy injection", `{"sortBy":"created_on; drop table"}`, 400},
		{"malformed json", `{"limit":`, 400},
		{"not an object", `[1,2]`, 400},
		{"wrong type", `{"limit":"ten"}`, 400},
		{"unknown field", `{"authorId":"x"}`, 400},
		{"trailing data", `{} {}`, 400},
		{"body too large", `{"searchQuery":"` + strings.Repeat("a", 20<<10) + `"}`, 413},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeEntity{}
			rec := do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus != 200 && len(f.searchCalls) != 0 {
				t.Error("upstream called for a rejected request")
			}
		})
	}
}

func TestSearchDefaultLimit(t *testing.T) {
	f := &fakeEntity{}
	do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", `{}`)
	if got := f.searchCalls[0].Pagination.Limit; got != defaultLimit {
		t.Errorf("limit = %d, want %d", got, defaultLimit)
	}
}

func TestSearchUpstreamErrorMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"upstream 400 is 502, not relayed", &apierror.Error{StatusCode: 400, Body: "unknown field sortBy"}, 502},
		{"upstream 404 is 502 on search", &apierror.Error{StatusCode: 404}, 502},
		{"upstream 503", &apierror.Error{StatusCode: 503}, 502},
		{"timeout", context.DeadlineExceeded, 504},
		{"wrapped timeout", errors.Join(errors.New("x"), context.DeadlineExceeded), 504},
		{"other", errors.New("boom"), 502},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeEntity{searchErr: tt.err}
			rec := do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", `{}`)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if strings.Contains(rec.Body.String(), "sortBy") || strings.Contains(rec.Body.String(), "boom") {
				t.Errorf("upstream detail relayed: %s", rec.Body)
			}
		})
	}
}

func TestListKnowledgeBasesOnlyActive(t *testing.T) {
	f := &fakeEntity{kbs: []entity.KnowledgeBase{
		{ID: "kb-1", Title: "One", Active: true},
		{ID: "kb-2", Title: "Hidden", Active: false},
		{ID: "bad/id", Title: "Weird", Active: true},
	}}
	rec := do(newMux(f, nil), http.MethodGet, "/public/knowledge-bases", "")
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var got listKnowledgeBasesDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.KnowledgeBases) != 1 || got.KnowledgeBases[0].ID != "kb-1" || got.KnowledgeBases[0].Title != "One" {
		t.Errorf("got %+v", got.KnowledgeBases)
	}
	// Whitelist: id and title only, nothing else (no active flag, no timestamps).
	var raw struct {
		KnowledgeBases []map[string]any `json:"knowledgeBases"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	if len(raw.KnowledgeBases) != 1 || len(raw.KnowledgeBases[0]) != 2 {
		t.Errorf("knowledge base DTO is not id+title only: %s", rec.Body)
	}

	f = &fakeEntity{kbsErr: &apierror.Error{StatusCode: 500, Body: "pg down"}}
	rec = do(newMux(f, nil), http.MethodGet, "/public/knowledge-bases", "")
	if rec.Code != 502 || strings.Contains(rec.Body.String(), "pg down") {
		t.Errorf("status %d body %s", rec.Code, rec.Body)
	}
}

func TestRelatedRequiresPublishedSource(t *testing.T) {
	for _, state := range []string{"draft", "pending_review", "retired"} {
		f := &fakeEntity{articles: map[string]entity.KBArticle{uuidA: article(uuidA, state)}}
		sim := fakeSimilarity{matches: []pinecone.Match{{ID: uuidB, Score: 0.9}}}
		rec := do(newMux(f, sim), http.MethodGet, "/public/kb-articles/"+uuidA+"/related", "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", state, rec.Code)
		}
		if len(f.getIDs) != 1 || len(f.searchCalls) != 0 {
			t.Errorf("%s: lookups after a rejected source: get=%v search=%d", state, f.getIDs, len(f.searchCalls))
		}
	}
	rec := do(newMux(&fakeEntity{}, nil), http.MethodGet, "/public/kb-articles/../related", "")
	if rec.Code == http.StatusOK {
		t.Error("bad id should not succeed")
	}
	rec = do(newMux(&fakeEntity{}, nil), http.MethodGet, "/public/kb-articles/a%2Fb/related", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("encoded slash id: status = %d, want 400", rec.Code)
	}
}

func TestRelatedFiltersMatchesThroughPublishedLookup(t *testing.T) {
	f := &fakeEntity{articles: map[string]entity.KBArticle{
		uuidA: article(uuidA, "published"),
		uuidB: article(uuidB, "published"),
		uuidC: article(uuidC, "draft"),
		"dd":  article("dd", "published"),
	}}
	sim := fakeSimilarity{matches: []pinecone.Match{
		{ID: uuidA, Score: 1},           // the source itself
		{ID: uuidC, Score: 0.95},        // unpublished
		{ID: "ghost", Score: 0.9},       // unknown upstream
		{ID: "../cases/1", Score: 0.85}, // invalid id, never looked up
		{ID: uuidB, Score: 0.8},         // good
		{ID: uuidB, Score: 0.7},         // duplicate
		{ID: "dd", Score: 0.6},          // good
	}}
	rec := do(newMux(f, sim), http.MethodGet, "/public/kb-articles/"+uuidA+"/related", "")
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var got relatedArticlesDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Articles) != 2 || got.Articles[0].ID != uuidB || got.Articles[1].ID != "dd" {
		t.Fatalf("articles = %+v", got.Articles)
	}
	if got.Articles[0].Title != "Title "+uuidB || got.Articles[0].Score != 0.8 {
		t.Errorf("title must come from entity-service, score from the index: %+v", got.Articles[0])
	}
	for _, id := range f.getIDs {
		if !validID(id) {
			t.Errorf("invalid id %q reached upstream", id)
		}
	}
	if len(f.searchCalls) != 0 {
		t.Error("fallback used although semantic matches survived")
	}
}

func TestRelatedFallsBackToSameKnowledgeBase(t *testing.T) {
	search := entity.SearchResponse{Articles: []entity.KBArticle{
		article(uuidA, "published"), // source, excluded
		article(uuidB, "published"),
		article(uuidC, "draft"), // defensive: must not appear
	}}
	cases := []struct {
		name string
		sim  Similarity
	}{
		{"index disabled", nil},
		{"index error", fakeSimilarity{err: errors.New("down")}},
		{"index returns only unpublished", fakeSimilarity{matches: []pinecone.Match{{ID: uuidC, Score: 1}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeEntity{
				articles: map[string]entity.KBArticle{uuidA: article(uuidA, "published"), uuidC: article(uuidC, "draft")},
				search:   search,
			}
			rec := do(newMux(f, tc.sim), http.MethodGet, "/public/kb-articles/"+uuidA+"/related", "")
			if rec.Code != 200 {
				t.Fatalf("status %d", rec.Code)
			}
			var got relatedArticlesDTO
			_ = json.Unmarshal(rec.Body.Bytes(), &got)
			if len(got.Articles) != 1 || got.Articles[0].ID != uuidB {
				t.Fatalf("articles = %+v", got.Articles)
			}
			if len(f.searchCalls) != 1 || f.searchCalls[0].States[0] != "published" || f.searchCalls[0].KnowledgeBaseID != "kb-1" {
				t.Errorf("fallback search = %+v", f.searchCalls)
			}
		})
	}
}

func TestRelatedFallbackFailureIsEmptyList(t *testing.T) {
	f := &fakeEntity{
		articles:  map[string]entity.KBArticle{uuidA: article(uuidA, "published")},
		searchErr: errors.New("down"),
	}
	rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/"+uuidA+"/related", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"articles":[]}` {
		t.Errorf("status %d body %s", rec.Code, rec.Body)
	}
}

func TestNotFoundAndHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	NotFound(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), `"message"`) {
		t.Errorf("NotFound: %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	Health(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != 200 || rec.Body.String() != `{"status":"ok"}` {
		t.Errorf("Health: %d %s", rec.Code, rec.Body)
	}
}

func ptr[T any](v T) *T { return &v }

// inactiveKBEntity has kb-1 active and kb-off inactive, with one published
// article in each, so every route can be checked against the inactive one.
func inactiveKBEntity() *fakeEntity {
	off := article(uuidB, "published")
	off.KnowledgeBaseID = "kb-off"
	return &fakeEntity{
		kbs: []entity.KnowledgeBase{
			{ID: "kb-1", Title: "One", Active: true},
			{ID: "kb-off", Title: "Off", Active: false},
		},
		articles: map[string]entity.KBArticle{uuidA: article(uuidA, "published"), uuidB: off},
	}
}

func TestInactiveKnowledgeBaseIsNotServed(t *testing.T) {
	t.Run("get article in inactive kb is 404", func(t *testing.T) {
		rec := do(newMux(inactiveKBEntity(), nil), http.MethodGet, "/public/kb-articles/"+uuidB, "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})
	t.Run("get article in active kb is 200", func(t *testing.T) {
		rec := do(newMux(inactiveKBEntity(), nil), http.MethodGet, "/public/kb-articles/"+uuidA, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})
	t.Run("search with inactive knowledgeBaseId is 404 without calling search", func(t *testing.T) {
		f := inactiveKBEntity()
		rec := do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", `{"knowledgeBaseId":"kb-off"}`)
		if rec.Code != http.StatusNotFound || len(f.searchCalls) != 0 {
			t.Fatalf("status = %d, searchCalls = %d", rec.Code, len(f.searchCalls))
		}
	})
	t.Run("search with unknown knowledgeBaseId is 404", func(t *testing.T) {
		rec := do(newMux(inactiveKBEntity(), nil), http.MethodPost, "/public/kb-articles/search", `{"knowledgeBaseId":"nope"}`)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})
	t.Run("search without knowledgeBaseId drops inactive-kb articles", func(t *testing.T) {
		f := inactiveKBEntity()
		off := f.articles[uuidB]
		f.search = entity.SearchResponse{Articles: []entity.KBArticle{article(uuidA, "published"), off}, Total: 2, Limit: 20}
		rec := do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", `{}`)
		if rec.Code != 200 {
			t.Fatalf("status = %d", rec.Code)
		}
		var got struct {
			Articles []map[string]any `json:"articles"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if len(got.Articles) != 1 || got.Articles[0]["id"] != uuidA {
			t.Errorf("articles = %s", rec.Body)
		}
	})
	t.Run("related for a source in an inactive kb is 404", func(t *testing.T) {
		f := inactiveKBEntity()
		rec := do(newMux(f, fakeSimilarity{matches: []pinecone.Match{{ID: uuidA, Score: 1}}}), http.MethodGet, "/public/kb-articles/"+uuidB+"/related", "")
		if rec.Code != http.StatusNotFound || len(f.searchCalls) != 0 {
			t.Fatalf("status = %d, searchCalls = %d", rec.Code, len(f.searchCalls))
		}
	})
	t.Run("related drops matches in an inactive kb", func(t *testing.T) {
		f := inactiveKBEntity()
		third := article(uuidC, "published")
		f.articles[uuidC] = third
		sim := fakeSimilarity{matches: []pinecone.Match{{ID: uuidB, Score: 0.9}, {ID: uuidC, Score: 0.8}}}
		rec := do(newMux(f, sim), http.MethodGet, "/public/kb-articles/"+uuidA+"/related", "")
		var got relatedArticlesDTO
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if rec.Code != 200 || len(got.Articles) != 1 || got.Articles[0].ID != uuidC {
			t.Fatalf("status %d articles %s", rec.Code, rec.Body)
		}
	})
	t.Run("related fallback drops inactive-kb articles", func(t *testing.T) {
		f := inactiveKBEntity()
		off := f.articles[uuidB]
		same := article(uuidC, "published")
		f.search = entity.SearchResponse{Articles: []entity.KBArticle{off, same}}
		rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/"+uuidA+"/related", "")
		var got relatedArticlesDTO
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if len(got.Articles) != 1 || got.Articles[0].ID != uuidC {
			t.Fatalf("articles %s", rec.Body)
		}
	})
}

func TestActiveKBSetFailsClosed(t *testing.T) {
	routes := []struct{ name, method, target, body string }{
		{"list", http.MethodGet, "/public/knowledge-bases", ""},
		{"search", http.MethodPost, "/public/kb-articles/search", `{}`},
		{"get", http.MethodGet, "/public/kb-articles/" + uuidA, ""},
		{"related", http.MethodGet, "/public/kb-articles/" + uuidA + "/related", ""},
	}
	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			f := &fakeEntity{
				kbsErr:   &apierror.Error{StatusCode: 500, Body: "pg down"},
				articles: map[string]entity.KBArticle{uuidA: article(uuidA, "published")},
			}
			rec := do(newMux(f, nil), rt.method, rt.target, rt.body)
			if rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), "pg down") {
				t.Fatalf("status = %d body %s, want generic 502", rec.Code, rec.Body)
			}
		})
	}
	t.Run("draft is 404 even if the set cannot load", func(t *testing.T) {
		f := &fakeEntity{kbsErr: errors.New("down"), articles: map[string]entity.KBArticle{uuidA: article(uuidA, "draft")}}
		rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/"+uuidA, "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestActiveKBCacheTTLAndFailureNotCached(t *testing.T) {
	now := time.Unix(1000, 0)
	f := &fakeEntity{articles: map[string]entity.KBArticle{uuidA: article(uuidA, "published")}}
	h := NewKBHandler(f, nil)
	h.activeKBs.now = func() time.Time { return now }
	get := func() int {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /public/kb-articles/{id}", h.GetArticle)
		return do(mux, http.MethodGet, "/public/kb-articles/"+uuidA, "").Code
	}
	if get() != 200 || get() != 200 {
		t.Fatal("want 200")
	}
	if f.kbCalls != 1 {
		t.Fatalf("kb list calls within TTL = %d, want 1", f.kbCalls)
	}
	now = now.Add(activeKBTTL + time.Second)
	// kb-1 is deactivated upstream: after the TTL the article stops being served.
	f.mu.Lock()
	f.kbs = []entity.KnowledgeBase{{ID: "kb-1", Title: "One", Active: false}}
	f.mu.Unlock()
	if got := get(); got != http.StatusNotFound {
		t.Fatalf("after TTL status = %d, want 404", got)
	}
	if f.kbCalls != 2 {
		t.Fatalf("kb list calls = %d, want 2", f.kbCalls)
	}
	// A failed refresh is not cached and not served stale.
	now = now.Add(activeKBTTL + time.Second)
	f.mu.Lock()
	f.kbsErr = errors.New("down")
	f.mu.Unlock()
	if got := get(); got != http.StatusBadGateway {
		t.Fatalf("failed refresh status = %d, want 502", got)
	}
	f.mu.Lock()
	f.kbsErr = nil
	f.kbs = []entity.KnowledgeBase{{ID: "kb-1", Title: "One", Active: true}}
	f.mu.Unlock()
	if got := get(); got != 200 {
		t.Fatalf("recovery status = %d, want 200", got)
	}
}

func TestActiveKBCacheConcurrent(t *testing.T) {
	f := &fakeEntity{articles: map[string]entity.KBArticle{uuidA: article(uuidA, "published")}}
	mux := newMux(f, nil)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rec := do(mux, http.MethodGet, "/public/kb-articles/"+uuidA, ""); rec.Code != 200 {
				t.Errorf("status %d", rec.Code)
			}
		}()
	}
	wg.Wait()
	if f.kbCalls < 1 || f.kbCalls > 50 {
		t.Errorf("kb list calls = %d", f.kbCalls)
	}
}

func TestSearchKnowledgeBaseIDValidatedBeforeUpstream(t *testing.T) {
	for _, id := range []string{"../x", "a/b", "a%2Fb", "a b", strings.Repeat("a", 65)} {
		f := &fakeEntity{}
		body, _ := json.Marshal(map[string]string{"knowledgeBaseId": id})
		rec := do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", string(body))
		if rec.Code != http.StatusBadRequest || len(f.searchCalls) != 0 || f.kbCalls != 0 {
			t.Errorf("%q: status %d searchCalls %d kbCalls %d", id, rec.Code, len(f.searchCalls), f.kbCalls)
		}
	}
}

func TestGetArticleTraversalIDIs400(t *testing.T) {
	f := &fakeEntity{}
	rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/%2F..%2Fcases%2F1", "")
	if rec.Code != http.StatusBadRequest || len(f.getIDs) != 0 {
		t.Fatalf("status %d getIDs %v", rec.Code, f.getIDs)
	}
}
