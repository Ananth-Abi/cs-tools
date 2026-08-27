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
	"log/slog"
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
	// the product passed to SendIncidentAlert. The reserved value
	// "default" (see defaultChatSpaceProduct) opts a space in as sendCard's
	// fallback for any product with no space of its own.
	Product string `json:"product"`
	// WebhookURL is that space's incoming webhook URL (Space settings > Apps
	// & integrations > Webhooks). It already carries its own key/token query
	// parameters, so no separate auth flow is needed.
	WebhookURL string `json:"webhookUrl"`
}

// defaultChatSpaceProduct is the reserved GoogleChatSpace.Product value
// (matched the same case/whitespace-insensitive way as any other product)
// that sendCard falls back to when the resolved product has no matching
// configured space at all, instead of erroring. Configure it by adding a
// {"product":"default","webhookUrl":"..."} entry to GOOGLE_CHAT_SPACES —
// entirely optional; with no such entry, an unmatched product still errors
// exactly as before.
//
// This is distinct from Dispatcher.defaultChatProduct
// (internal/dispatch), which only kicks in when a payload's own Product
// field is empty in the first place — that's a business-logic fallback for
// "the publisher didn't say," resolved before this client is ever called.
// defaultChatSpaceProduct instead covers a non-empty, real product that
// simply has no GOOGLE_CHAT_SPACES entry of its own (e.g. entity-service's
// case.created now sends a deployed product's actual display name, which
// won't match an operator's existing short config keys — like
// "api-manager" — until GOOGLE_CHAT_SPACES is updated to match; until it
// is, this fallback keeps every case.created/case.acknowledged alert
// landing somewhere instead of being dropped/retried/dead-lettered).
const defaultChatSpaceProduct = "default"

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

// Header is a pointer, unlike every other field on this type — every
// case.*/incident.created card sets it today, but a pointer keeps a
// header-less card representable (omitempty) rather than requiring an
// empty title on the wire, should a future card need one.
type chatCard struct {
	Header   *chatCardHeader   `json:"header,omitempty"`
	Sections []chatCardSection `json:"sections"`
}

type chatCardHeader struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
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

// chatHeaderCaseRef builds a case.*-card header's Title: the case's own
// identifiers, which read more prominently than a case title (a Roboto Mono
// -styled 15px header, per the redesign this matches) — "<caseNumber> ·
// <wso2CaseID>", falling back to caseNumber alone when the publisher didn't
// send a WSO2 case id.
func chatHeaderCaseRef(caseNumber, wso2CaseID string) string {
	if wso2CaseID == "" {
		return caseNumber
	}
	return caseNumber + " · " + wso2CaseID
}

// chatMetaLine joins parts with " · ", skipping any empty ones — the
// case.*-card redesign's "keep related facts on one line" convention
// (severity/product/team, or old/new severity/team), rather than one
// stacked row per fact.
func chatMetaLine(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " · ")
}

// SendCaseCreatedAlert posts a card message announcing a newly created
// case, to the Google Chat space configured for product. The case's own
// identifiers (caseNumber/wso2CaseID) lead the card as the header title;
// title (the case subject) is the header subtitle. The body is a single
// "New case" line — the one that distinguishes this alert from
// SendCaseAcknowledgedAlert/SendSeverityChangedAlert's cards, which don't
// carry it — followed by one more line combining severity (colored),
// productName, and team, each entirely omitted (not a blank slot) when the
// publisher didn't send it. "Open in CSM" and "Acknowledge" are real
// button widgets, not markup links; both still point at the same caseLink
// for now — dispatch.handleCaseCreated has no interactive card action to
// send "acknowledge" to yet, so this is a view link like the other one
// rather than a dead end. There is deliberately no team/codename line
// above the header — an earlier version of this alert had one, discarded
// per explicit product decision; the case reference now leads instead.
func (c *GoogleChatClient) SendCaseCreatedAlert(ctx context.Context, product, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink string) error {
	if caseNumber == "" {
		return fmt.Errorf("notifications: caseNumber is required")
	}
	sevPart := ""
	if severityLabel != "" {
		sevPart = caseAlertLine(`<font color="%s"><b>%s</b></font>`, severityColor, severityLabel)
	}
	lines := []string{`<b>New case</b>`}
	if meta := chatMetaLine(sevPart, caseAlertLine(`%s`, productName), caseAlertLine(`%s`, team)); meta != "" {
		lines = append(lines, meta)
	}
	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "case-created-alert",
				Card: chatCard{
					Header: &chatCardHeader{Title: chatHeaderCaseRef(caseNumber, wso2CaseID), Subtitle: title},
					Sections: []chatCardSection{
						{Widgets: []chatCardWidget{
							{TextParagraph: &chatTextParagraph{Text: strings.Join(lines, "<br>")}},
							{ButtonList: &chatButtonList{Buttons: []chatButton{
								{Text: "Open in CSM", OnClick: chatOnClick{OpenLink: chatOpenLink{URL: caseLink}}},
								{Text: "Acknowledge", OnClick: chatOnClick{OpenLink: chatOpenLink{URL: caseLink}}},
							}}},
						}},
					},
				},
			},
		},
	}
	return c.sendCard(ctx, product, msg)
}

// SendCaseAcknowledgedAlert posts a card message announcing that a case was
// acknowledged, to the same Google Chat space as its case.created alert.
// The case reference leads the header, same as every other case.* card;
// who acknowledged it is the header subtitle ("Ack by <name>"). The body
// is just team, entirely omitted (no body section at all) when the
// publisher didn't send one — the shortest of the three case.* cards. A
// single "View case" button links to caseLink, an addition over this
// alert's earlier link-free version.
func (c *GoogleChatClient) SendCaseAcknowledgedAlert(ctx context.Context, product, severityLabel, severityColor, caseNumber, wso2CaseID, caseLink, acknowledgerName, team string) error {
	if caseNumber == "" || acknowledgerName == "" {
		return fmt.Errorf("notifications: caseNumber and acknowledgerName are required")
	}
	sections := []chatCardSection{}
	if team != "" {
		sections = append(sections, chatCardSection{Widgets: []chatCardWidget{
			{TextParagraph: &chatTextParagraph{Text: caseAlertLine(`%s`, team)}},
		}})
	}
	sections = append(sections, chatCardSection{Widgets: []chatCardWidget{
		{ButtonList: &chatButtonList{Buttons: []chatButton{
			{Text: "View case", OnClick: chatOnClick{OpenLink: chatOpenLink{URL: caseLink}}},
		}}},
	}})
	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "case-acknowledged-alert",
				Card: chatCard{
					Header:   &chatCardHeader{Title: chatHeaderCaseRef(caseNumber, wso2CaseID), Subtitle: "Ack by " + acknowledgerName},
					Sections: sections,
				},
			},
		},
	}
	return c.sendCard(ctx, product, msg)
}

// SendSeverityChangedAlert posts a card message announcing a case's
// severity changed, to the same Google Chat space as its case.created
// alert. The case reference leads the header, same as every other case.*
// card; title (the case subject) is the header subtitle, omitted when the
// publisher didn't send one. The body is a single line: old severity, an
// arrow, new severity (colored — the case's current state, not the old
// one), and team, each entirely omitted when empty. A single "View case"
// button links to caseLink.
func (c *GoogleChatClient) SendSeverityChangedAlert(ctx context.Context, product, oldSeverityLabel, newSeverityLabel, newSeverityColor, caseNumber, wso2CaseID, title, team, caseLink string) error {
	if caseNumber == "" {
		return fmt.Errorf("notifications: caseNumber is required")
	}
	transition := ""
	if oldSeverityLabel != "" || newSeverityLabel != "" {
		transition = caseAlertLine(`%s → <font color="%s"><b>%s</b></font>`, oldSeverityLabel, newSeverityColor, newSeverityLabel)
	}
	text := chatMetaLine(transition, caseAlertLine(`%s`, team))

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "case-severity-changed-alert",
				Card: chatCard{
					Header: &chatCardHeader{Title: chatHeaderCaseRef(caseNumber, wso2CaseID), Subtitle: title},
					Sections: []chatCardSection{
						{Widgets: []chatCardWidget{
							{TextParagraph: &chatTextParagraph{Text: text}},
							{ButtonList: &chatButtonList{Buttons: []chatButton{
								{Text: "View case", OnClick: chatOnClick{OpenLink: chatOpenLink{URL: caseLink}}},
							}}},
						}},
					},
				},
			},
		},
	}
	return c.sendCard(ctx, product, msg)
}

// sendCard marshals msg and posts it to the webhook configured for
// product, shared by SendIncidentAlert/SendCaseCreatedAlert/
// SendCaseAcknowledgedAlert/SendSeverityChangedAlert.
func (c *GoogleChatClient) sendCard(ctx context.Context, product string, msg chatCardMessage) error {
	webhookURL, ok := c.webhookURLsByProduct[normalizeProduct(product)]
	if !ok || webhookURL == "" {
		fallbackURL, fbOK := c.webhookURLsByProduct[defaultChatSpaceProduct]
		if !fbOK || fallbackURL == "" {
			return fmt.Errorf("notifications: no google chat space configured for product %q", product)
		}
		slog.WarnContext(ctx, "notifications: no google chat space configured for product; falling back to the default space", "product", product)
		webhookURL = fallbackURL
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
