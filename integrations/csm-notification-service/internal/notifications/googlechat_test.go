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

package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
)

func TestSendIncidentAlert_ValidatesArgumentsBeforeCallingUpstream(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Run("rejects unconfigured product", func(t *testing.T) {
		c := NewGoogleChatClient(GoogleChatConfig{})
		if err := c.SendIncidentAlert(context.Background(), "api-manager", "title", "desc", "https://example.com"); err == nil {
			t.Fatal("expected error for unconfigured product, got nil")
		}
	})

	t.Run("rejects empty title", func(t *testing.T) {
		c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})
		if err := c.SendIncidentAlert(context.Background(), "api-manager", "", "desc", "https://example.com"); err == nil {
			t.Fatal("expected error for empty title, got nil")
		}
	})

	if called {
		t.Error("upstream should not have been called for invalid arguments")
	}
}

func TestSendIncidentAlert_RoutesToTheConfiguredProductsSpace(t *testing.T) {
	var apimCalled, isCalled bool
	apimSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apimCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer apimSrv.Close()
	isSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer isSrv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{
		{Product: "API-Manager", WebhookURL: apimSrv.URL},
		{Product: "identity-server", WebhookURL: isSrv.URL},
	}})

	// Product matching is case- and whitespace-insensitive.
	if err := c.SendIncidentAlert(context.Background(), " api-manager ", "title", "desc", "https://example.com"); err != nil {
		t.Fatalf("SendIncidentAlert returned error: %v", err)
	}
	if !apimCalled || isCalled {
		t.Errorf("apimCalled = %v, isCalled = %v, want true, false", apimCalled, isCalled)
	}

	if err := c.SendIncidentAlert(context.Background(), "unknown-product", "title", "desc", "https://example.com"); err == nil {
		t.Fatal("expected error for a product with no configured space, got nil")
	}
}

func TestNewGoogleChatClient_MarksDuplicateNormalizedProductsUnconfigured(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{
		{Product: "API-Manager", WebhookURL: "https://chat.example.com/a"},
		{Product: " api-manager ", WebhookURL: "https://chat.example.com/b"},
	}})

	err := c.SendIncidentAlert(context.Background(), "api-manager", "title", "desc", "https://example.com")
	if err == nil {
		t.Fatal("expected error for a product with a duplicate (now unconfigured) mapping, got nil")
	}
	if !strings.Contains(err.Error(), "no google chat space configured") {
		t.Errorf("expected unconfigured space error, got: %v", err)
	}
}

// TestSendCard_FallsBackToDefaultSpaceWhenProductUnconfigured verifies the
// opt-in "default" GOOGLE_CHAT_SPACES entry: a product with no space of its
// own routes to the space configured under the reserved "default" key
// instead of erroring.
func TestSendCard_FallsBackToDefaultSpaceWhenProductUnconfigured(t *testing.T) {
	var defaultCalled bool
	defaultSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defaultCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer defaultSrv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{
		{Product: "api-manager", WebhookURL: "https://chat.example.com/apim"},
		{Product: "default", WebhookURL: defaultSrv.URL},
	}})

	if err := c.SendIncidentAlert(context.Background(), "identity-server", "title", "desc", "https://example.com"); err != nil {
		t.Fatalf("SendIncidentAlert returned error: %v", err)
	}
	if !defaultCalled {
		t.Error("expected the alert to be posted to the default space's webhook, but it wasn't called")
	}
}

// TestSendCard_MatchedProductTakesPriorityOverDefault verifies the default
// space fallback never overrides a product that does have its own
// configured space.
func TestSendCard_MatchedProductTakesPriorityOverDefault(t *testing.T) {
	var apimCalled, defaultCalled bool
	apimSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apimCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer apimSrv.Close()
	defaultSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defaultCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer defaultSrv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{
		{Product: "api-manager", WebhookURL: apimSrv.URL},
		{Product: "default", WebhookURL: defaultSrv.URL},
	}})

	if err := c.SendIncidentAlert(context.Background(), "api-manager", "title", "desc", "https://example.com"); err != nil {
		t.Fatalf("SendIncidentAlert returned error: %v", err)
	}
	if !apimCalled || defaultCalled {
		t.Errorf("apimCalled = %v, defaultCalled = %v, want true, false", apimCalled, defaultCalled)
	}
}

// TestSendCard_StillErrorsWithNoDefaultConfigured verifies the fallback is
// opt-in: with no "default" GOOGLE_CHAT_SPACES entry at all, an unmatched
// product still errors exactly as before this change.
func TestSendCard_StillErrorsWithNoDefaultConfigured(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{
		{Product: "api-manager", WebhookURL: "https://chat.example.com/apim"},
	}})

	err := c.SendIncidentAlert(context.Background(), "identity-server", "title", "desc", "https://example.com")
	if err == nil {
		t.Fatal("expected error for an unmatched product with no default space configured, got nil")
	}
	if !strings.Contains(err.Error(), "no google chat space configured") {
		t.Errorf("expected unconfigured space error, got: %v", err)
	}
}

func TestSendIncidentAlert_RedactsWebhookURLOnNetworkFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	webhookURL := srv.URL + "/messages?key=SECRET_KEY&token=SECRET_TOKEN"
	srv.Close() // subsequent requests to webhookURL now fail to connect

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: webhookURL}}})

	err := c.SendIncidentAlert(context.Background(), "api-manager", "title", "desc", "https://example.com")
	if err == nil {
		t.Fatal("expected an error from an unreachable webhook, got nil")
	}
	if strings.Contains(err.Error(), "SECRET_KEY") || strings.Contains(err.Error(), "SECRET_TOKEN") {
		t.Errorf("error leaked webhook credentials: %v", err)
	}
}

func TestSendIncidentAlert_SendsExpectedCard(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendIncidentAlert(context.Background(), "api-manager", "P1 Incident - CASE-001", "Asgardeo Add Customer", "https://portal.example.com/operations/incidents/CASE-001")
	if err != nil {
		t.Fatalf("SendIncidentAlert returned error: %v", err)
	}

	if len(capturedBody.CardsV2) != 1 {
		t.Fatalf("CardsV2 length = %d, want 1", len(capturedBody.CardsV2))
	}
	card := capturedBody.CardsV2[0].Card
	if card.Header.Title != "P1 Incident - CASE-001" {
		t.Errorf("Header.Title = %q, want %q", card.Header.Title, "P1 Incident - CASE-001")
	}
	if len(card.Sections) != 2 {
		t.Fatalf("Sections length = %d, want 2", len(card.Sections))
	}
	if card.Sections[0].Header != "Short Description" {
		t.Errorf("Sections[0].Header = %q, want %q", card.Sections[0].Header, "Short Description")
	}
	if got := card.Sections[0].Widgets[0].TextParagraph.Text; got != "Asgardeo Add Customer" {
		t.Errorf("short description text = %q, want %q", got, "Asgardeo Add Customer")
	}
	button := card.Sections[1].Widgets[0].ButtonList.Buttons[0]
	if button.Text != "Open in CSM Portal" {
		t.Errorf("button text = %q, want %q", button.Text, "Open in CSM Portal")
	}
	if button.OnClick.OpenLink.URL != "https://portal.example.com/operations/incidents/CASE-001" {
		t.Errorf("button URL = %q, want %q", button.OnClick.OpenLink.URL, "https://portal.example.com/operations/incidents/CASE-001")
	}
}

func TestSendIncidentAlert_MapsUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid webhook payload"})
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendIncidentAlert(context.Background(), "api-manager", "title", "desc", "https://example.com")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *apierror.Error, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusBadRequest)
	}
}

func TestSendIncidentAlert_ConstructsWithZeroValueConfig(t *testing.T) {
	// NewGoogleChatClient must never fail or panic even when the Google Chat
	// channel has not been configured for a given deployment.
	c := NewGoogleChatClient(GoogleChatConfig{})
	if c == nil {
		t.Fatal("NewGoogleChatClient returned nil for zero-value GoogleChatConfig")
	}
}

// TestSendCaseCreatedAlert_SendsExpectedCard verifies the case.created Chat
// card's actual shape: no header at all (see chatCard.Header's own doc
// comment), a single TextParagraph widget whose text is every line
// <br>-joined in order — colored severity, linked case number, WSO2 case
// reference, product, title, then the two "Open in CSM"/"ACKNOWLEDGE CASE"
// links — and that a dynamic value containing HTML-significant characters
// is escaped rather than allowed to break the surrounding markup.
func TestSendCaseCreatedAlert_SendsExpectedCard(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendCaseCreatedAlert(context.Background(), "api-manager",
		"Critical (P1)", "#DC2626", "CS0001001", "WSO2-1000", "WSO2 API Manager",
		`Tom & Jerry <script>`, "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendCaseCreatedAlert returned error: %v", err)
	}

	if len(capturedBody.CardsV2) != 1 {
		t.Fatalf("CardsV2 length = %d, want 1", len(capturedBody.CardsV2))
	}
	card := capturedBody.CardsV2[0].Card
	if card.Header != nil {
		t.Errorf("Header = %+v, want nil (no header on this card)", card.Header)
	}
	if len(card.Sections) != 1 || len(card.Sections[0].Widgets) != 1 {
		t.Fatalf("unexpected sections/widgets shape: %+v", card.Sections)
	}
	text := card.Sections[0].Widgets[0].TextParagraph.Text
	want := strings.Join([]string{
		`<font color="#DC2626"><b>Critical (P1)</b></font>`,
		`<a href="https://csm.example.com/cases/CASE-1">CS0001001</a>`,
		`<b>WSO2-1000</b>`,
		`<b>WSO2 API Manager</b>`,
		`Tom &amp; Jerry &lt;script&gt;`,
		`<a href="https://csm.example.com/cases/CASE-1">Open in CSM</a>`,
		`<a href="https://csm.example.com/cases/CASE-1">ACKNOWLEDGE CASE</a>`,
	}, "<br>")
	if text != want {
		t.Errorf("card text =\n%q\nwant\n%q", text, want)
	}
}

// TestSendCaseCreatedAlert_OmitsEmptyOptionalLines verifies wso2CaseID/
// productName/title are each dropped from the card entirely (not rendered
// as an empty line) when the caller doesn't supply one.
func TestSendCaseCreatedAlert_OmitsEmptyOptionalLines(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendCaseCreatedAlert(context.Background(), "api-manager",
		"Medium (P3)", "#7C3AED", "CS0001001", "", "", "", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendCaseCreatedAlert returned error: %v", err)
	}

	text := capturedBody.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text
	if strings.Count(text, "<br>") != 3 {
		t.Errorf("expected exactly 4 lines (severity, case number, Open in CSM, ACKNOWLEDGE CASE), got text = %q", text)
	}
}

func TestSendCaseCreatedAlert_RejectsEmptyCaseNumber(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: "https://example.com"}}})
	if err := c.SendCaseCreatedAlert(context.Background(), "api-manager", "Critical (P1)", "#DC2626", "", "WSO2-1000", "WSO2 API Manager", "title", "https://example.com/cases/1"); err == nil {
		t.Fatal("expected error for empty caseNumber, got nil")
	}
}

// TestSendCaseAcknowledgedAlert_SendsExpectedCard verifies the
// case.acknowledged Chat card's shape: no header, a single TextParagraph
// line matching an existing internal WSO2-support Chat format —
// "<severity> <caseNumber> <wso2CaseID>: Ack by <name>".
func TestSendCaseAcknowledgedAlert_SendsExpectedCard(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendCaseAcknowledgedAlert(context.Background(), "api-manager",
		"Critical (P1)", "#DC2626", "CS0001002", "WSO2-1001", "https://csm.example.com/cases/CASE-1", "Jane Doe")
	if err != nil {
		t.Fatalf("SendCaseAcknowledgedAlert returned error: %v", err)
	}

	card := capturedBody.CardsV2[0].Card
	if card.Header != nil {
		t.Errorf("Header = %+v, want nil", card.Header)
	}
	text := card.Sections[0].Widgets[0].TextParagraph.Text
	want := `<font color="#DC2626"><b>Critical (P1)</b></font> <a href="https://csm.example.com/cases/CASE-1">CS0001002</a> WSO2-1001: Ack by Jane Doe`
	if text != want {
		t.Errorf("card text = %q, want %q", text, want)
	}
}

func TestSendCaseAcknowledgedAlert_OmitsEmptyWSO2CaseID(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendCaseAcknowledgedAlert(context.Background(), "api-manager",
		"High (P2)", "#EA580C", "CS0001003", "", "https://csm.example.com/cases/CASE-1", "Jane Doe")
	if err != nil {
		t.Fatalf("SendCaseAcknowledgedAlert returned error: %v", err)
	}

	text := capturedBody.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text
	want := `<font color="#EA580C"><b>High (P2)</b></font> <a href="https://csm.example.com/cases/CASE-1">CS0001003</a>: Ack by Jane Doe`
	if text != want {
		t.Errorf("card text = %q, want %q", text, want)
	}
}

func TestSendCaseAcknowledgedAlert_RejectsMissingRequiredArgs(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: "https://example.com"}}})
	if err := c.SendCaseAcknowledgedAlert(context.Background(), "api-manager", "Critical (P1)", "#DC2626", "", "WSO2-1000", "https://example.com/cases/1", "Jane Doe"); err == nil {
		t.Fatal("expected error for empty caseNumber, got nil")
	}
	if err := c.SendCaseAcknowledgedAlert(context.Background(), "api-manager", "Critical (P1)", "#DC2626", "CS0001", "WSO2-1000", "https://example.com/cases/1", ""); err == nil {
		t.Fatal("expected error for empty acknowledgerName, got nil")
	}
}
