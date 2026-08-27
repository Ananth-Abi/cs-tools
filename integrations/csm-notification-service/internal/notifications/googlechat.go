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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
)

// GoogleChatSpace maps a single product to the Google Chat space that should
// receive its incident alerts.
type GoogleChatSpace struct {
	// Product identifies the product this space is dedicated to (e.g.
	// "api-manager", "identity-server"). Matched case-insensitively against
	// the product passed to SendIncidentAlert.
	Product string `json:"product"`
	// WebhookURL is that space's incoming webhook URL (Space settings > Apps
	// & integrations > Webhooks). It already carries its own key/token query
	// parameters, so no separate auth flow is needed.
	WebhookURL string `json:"webhookUrl"`
}

// GoogleChatConfig holds the configuration for the Google Chat notification
// channel: one space per product, since each WSO2 product has its own space.
type GoogleChatConfig struct {
	Spaces []GoogleChatSpace
}

// GoogleChatClient posts messages to a Google Chat space via an incoming
// webhook, routing each alert to the space configured for the case's
// product. Unlike the OAuth2-authenticated clients in this package, a
// webhook URL is the only credential required.
//
// NewGoogleChatClient never fails, so it is safe to construct with a
// zero-value GoogleChatConfig (e.g. when this channel is not yet configured
// for a given deployment) — a missing or unmatched product only surfaces as
// an error the first time SendIncidentAlert is called for it.
type GoogleChatClient struct {
	http                 *http.Client
	webhookURLsByProduct map[string]string
}

// NewGoogleChatClient constructs a GoogleChatClient that routes alerts to the
// webhook configured for each product in cfg.Spaces.
func NewGoogleChatClient(cfg GoogleChatConfig) *GoogleChatClient {
	webhookURLsByProduct := make(map[string]string, len(cfg.Spaces))
	for _, space := range cfg.Spaces {
		product := normalizeProduct(space.Product)
		if product == "" || strings.TrimSpace(space.WebhookURL) == "" {
			continue
		}
		// A second space normalizing to the same product (e.g. "API-Manager"
		// and " api-manager ") is a configuration mistake — mark it
		// unconfigured rather than silently routing to whichever URL came
		// last.
		if _, exists := webhookURLsByProduct[product]; exists {
			webhookURLsByProduct[product] = ""
			continue
		}
		webhookURLsByProduct[product] = space.WebhookURL
	}
	return &GoogleChatClient{
		http:                 &http.Client{Timeout: 10 * time.Second},
		webhookURLsByProduct: webhookURLsByProduct,
	}
}

// normalizeProduct makes product matching case- and whitespace-insensitive.
func normalizeProduct(product string) string {
	return strings.ToLower(strings.TrimSpace(product))
}

// redactURLError strips the request URL — which carries the webhook's secret
// key/token query parameters — out of a *url.Error before it's wrapped and
// potentially logged, keeping only the underlying (safe) failure reason.
func redactURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

// chatCardMessage is the wire shape Google Chat's webhook API expects for a
// single card message: https://developers.google.com/chat/api/guides/message-formats/cards
type chatCardMessage struct {
	CardsV2 []chatCardWrapper `json:"cardsV2"`
}

type chatCardWrapper struct {
	CardID string   `json:"cardId"`
	Card   chatCard `json:"card"`
}

// Header is a pointer, unlike every other field on this type, so a card
// with no header (case.created/case.acknowledged — see
// SendCaseCreatedAlert/SendCaseAcknowledgedAlert) can omit it from the wire
// entirely (omitempty) instead of sending an empty title. SendIncidentAlert
// always sets it, so its own output is unchanged by this.
type chatCard struct {
	Header   *chatCardHeader   `json:"header,omitempty"`
	Sections []chatCardSection `json:"sections"`
}

type chatCardHeader struct {
	Title string `json:"title"`
}

type chatCardSection struct {
	Header  string           `json:"header,omitempty"`
	Widgets []chatCardWidget `json:"widgets"`
}

// chatCardWidget is a union type: exactly one of TextParagraph or ButtonList
// is set per widget, matching Google Chat's widget schema.
type chatCardWidget struct {
	TextParagraph *chatTextParagraph `json:"textParagraph,omitempty"`
	ButtonList    *chatButtonList    `json:"buttonList,omitempty"`
}

type chatTextParagraph struct {
	Text string `json:"text"`
}

type chatButtonList struct {
	Buttons []chatButton `json:"buttons"`
}

type chatButton struct {
	Text    string      `json:"text"`
	OnClick chatOnClick `json:"onClick"`
}

type chatOnClick struct {
	OpenLink chatOpenLink `json:"openLink"`
}

type chatOpenLink struct {
	URL string `json:"url"`
}

// SendIncidentAlert posts a card message announcing a newly created
// incident/case, with a button linking back to the case in the CSM portal,
// to the Google Chat space configured for the given product.
func (c *GoogleChatClient) SendIncidentAlert(ctx context.Context, product, title, shortDescription, portalURL string) error {
	if title == "" {
		return fmt.Errorf("notifications: title is required")
	}
	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "incident-alert",
				Card: chatCard{
					Header: &chatCardHeader{Title: title},
					Sections: []chatCardSection{
						{
							Header: "Short Description",
							Widgets: []chatCardWidget{
								{TextParagraph: &chatTextParagraph{Text: shortDescription}},
							},
						},
						{
							Widgets: []chatCardWidget{
								{
									ButtonList: &chatButtonList{
										Buttons: []chatButton{
											{
												Text:    "Open in CSM Portal",
												OnClick: chatOnClick{OpenLink: chatOpenLink{URL: portalURL}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return c.sendCard(ctx, product, msg)
}

// caseAlertLine builds one <br>-joined line of a case.created/
// case.acknowledged Chat card's single TextParagraph, HTML-escaping label
// (the dynamic value) but not the markup surrounding it — mirrors
// internal/notifications' own escapeHTML reasoning for email templates:
// Google Chat's card text interprets a limited HTML subset (<b>, <font
// color="...">, <a href="...">), so a dynamic value that happened to
// contain "<" or "&" must not be allowed to break out of the tag it's
// placed in.
func caseAlertLine(format string, args ...any) string {
	escaped := make([]any, len(args))
	for i, a := range args {
		escaped[i] = html.EscapeString(fmt.Sprint(a))
	}
	return fmt.Sprintf(format, escaped...)
}

// SendCaseCreatedAlert posts a card message announcing a newly created case
// — like SendIncidentAlert, but a case.created-specific layout matching an
// existing internal WSO2-support Chat format that predates this service: a
// colored severity/priority line, the case number (linked) and WSO2 case
// reference, the case's product, its title, and two more links ("Open in
// CSM", "ACKNOWLEDGE CASE"). All three links point at the same caseLink for
// now — dispatch.handleCaseCreated doesn't yet have anywhere more specific
// to send "acknowledge" to from Chat (there's no interactive card action
// wired up), so it's a view link like the other two rather than a dead
// end. wso2CaseID/productName/title are each dropped from the card
// entirely (not rendered as an empty line) when the publisher didn't send
// one. There is deliberately no card header — an earlier version of this
// alert included a top-line team/codename, discarded per explicit product
// decision (no field in this service's data model corresponds to it,
// and Google Chat already shows the sending app's own name above the card).
func (c *GoogleChatClient) SendCaseCreatedAlert(ctx context.Context, product, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, caseLink string) error {
	if caseNumber == "" {
		return fmt.Errorf("notifications: caseNumber is required")
	}
	lines := []string{
		caseAlertLine(`<font color="%s"><b>%s</b></font>`, severityColor, severityLabel),
		caseAlertLine(`<a href="%s">%s</a>`, caseLink, caseNumber),
	}
	if wso2CaseID != "" {
		lines = append(lines, caseAlertLine(`<b>%s</b>`, wso2CaseID))
	}
	if productName != "" {
		lines = append(lines, caseAlertLine(`<b>%s</b>`, productName))
	}
	if title != "" {
		lines = append(lines, caseAlertLine(`%s`, title))
	}
	lines = append(lines,
		caseAlertLine(`<a href="%s">Open in CSM</a>`, caseLink),
		caseAlertLine(`<a href="%s">ACKNOWLEDGE CASE</a>`, caseLink),
	)
	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "case-created-alert",
				Card: chatCard{
					Sections: []chatCardSection{
						{Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: strings.Join(lines, "<br>")}}}},
					},
				},
			},
		},
	}
	return c.sendCard(ctx, product, msg)
}

// SendCaseAcknowledgedAlert posts a single-line card message announcing
// that a case was acknowledged, to the same Google Chat space as its
// case.created alert — matching an existing internal WSO2-support Chat
// format: "<severity (Pn)> <caseNumber> <wso2CaseID>: Ack by <name>".
// wso2CaseID is dropped from the line entirely when the publisher didn't
// send one, same reasoning as SendCaseCreatedAlert.
func (c *GoogleChatClient) SendCaseAcknowledgedAlert(ctx context.Context, product, severityLabel, severityColor, caseNumber, wso2CaseID, caseLink, acknowledgerName string) error {
	if caseNumber == "" || acknowledgerName == "" {
		return fmt.Errorf("notifications: caseNumber and acknowledgerName are required")
	}
	text := caseAlertLine(`<font color="%s"><b>%s</b></font> <a href="%s">%s</a>`, severityColor, severityLabel, caseLink, caseNumber)
	if wso2CaseID != "" {
		text += " " + caseAlertLine(`%s`, wso2CaseID)
	}
	text += caseAlertLine(`: Ack by %s`, acknowledgerName)

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "case-acknowledged-alert",
				Card: chatCard{
					Sections: []chatCardSection{
						{Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: text}}}},
					},
				},
			},
		},
	}
	return c.sendCard(ctx, product, msg)
}

// sendCard marshals msg and posts it to the webhook configured for
// product, shared by SendIncidentAlert/SendCaseCreatedAlert/
// SendCaseAcknowledgedAlert.
func (c *GoogleChatClient) sendCard(ctx context.Context, product string, msg chatCardMessage) error {
	webhookURL, ok := c.webhookURLsByProduct[normalizeProduct(product)]
	if !ok || webhookURL == "" {
		return fmt.Errorf("notifications: no google chat space configured for product %q", product)
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("notifications: encode google chat message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notifications: build google chat request: %w", redactURLError(err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("notifications: post google chat message: %w", redactURLError(err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("notifications: read google chat response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	return nil
}
