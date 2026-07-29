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
	_ "embed"
	"encoding/base64"
	"html"
	"strings"
)

//go:embed templates/wso2-logo.png
var wso2LogoPNG []byte

//go:embed templates/comment_added.html
var commentAddedTemplateRaw string

//go:embed templates/status_changed.html
var statusChangedTemplateRaw string

//go:embed templates/case_assigned.html
var caseAssignedTemplateRaw string

//go:embed templates/case_created.html
var caseCreatedTemplateRaw string

// bakeLogo substitutes the constant WSO2 logo — as an inline base64 data URI
// — into raw's <!-- [LOGO_SRC] --> placeholder. Done once per template at
// package init rather than on every Render* call, since the logo never
// varies between emails.
func bakeLogo(raw string) string {
	return strings.Replace(raw, "<!-- [LOGO_SRC] -->",
		"data:image/png;base64,"+base64.StdEncoding.EncodeToString(wso2LogoPNG), 1)
}

var (
	commentAddedTemplate  = bakeLogo(commentAddedTemplateRaw)
	statusChangedTemplate = bakeLogo(statusChangedTemplateRaw)
	caseAssignedTemplate  = bakeLogo(caseAssignedTemplateRaw)
	caseCreatedTemplate   = bakeLogo(caseCreatedTemplateRaw)
)

// escapeMultiline HTML-escapes s and converts its newlines to <br>, so
// free-text fields (a comment, a case description) keep their original line
// breaks when dropped into HTML.
func escapeMultiline(s string) string {
	return strings.ReplaceAll(html.EscapeString(s), "\n", "<br>")
}

// applyOptionalBlock handles a template section wrapped in
// "<!-- [BLOCK:<name>_START] -->"..."<!-- [BLOCK:<name>_END] -->": if value is
// empty, the whole section (markers included) is removed; otherwise only the
// markers are stripped, leaving the section's content in place for the
// caller's usual placeholder substitution. Returns tmpl unchanged if the
// markers aren't found.
func applyOptionalBlock(tmpl, name, value string) string {
	start := "<!-- [BLOCK:" + name + "_START] -->"
	end := "<!-- [BLOCK:" + name + "_END] -->"
	si := strings.Index(tmpl, start)
	ei := strings.Index(tmpl, end)
	if si == -1 || ei == -1 || ei < si {
		return tmpl
	}
	if strings.TrimSpace(value) == "" {
		return tmpl[:si] + tmpl[ei+len(end):]
	}
	return tmpl[:si] + tmpl[si+len(start):ei] + tmpl[ei+len(end):]
}

// RenderCommentAddedEmail fills in the "comment added" HTML email template.
// name, projectID, and caseTitle are HTML-escaped as-is; caseComment is
// HTML-escaped with newlines converted to <br> so the commenter's original
// line breaks are preserved. commentLink is the "Add Comment" call-to-action
// target; caseLink is the "View Case" link and the case-title link target.
func RenderCommentAddedEmail(name, projectID, caseTitle, caseComment, commentLink, caseLink string) string {
	replacer := strings.NewReplacer(
		"<!-- [NAME] -->", html.EscapeString(name),
		"<!-- [PROJECT_ID] -->", html.EscapeString(projectID),
		"<!-- [CASE_TITLE] -->", html.EscapeString(caseTitle),
		"<!-- [CASE_COMMENT] -->", escapeMultiline(caseComment),
		"<!-- [COMMENT_LINK] -->", html.EscapeString(commentLink),
		"<!-- [CASE_LINK] -->", html.EscapeString(caseLink),
	)
	return replacer.Replace(commentAddedTemplate)
}

// RenderStatusChangedEmail fills in the "case status changed" HTML email
// template. caseLink is used both for the case-ID link in the strap line and
// the "View Case" link; commentLink is the "Add Comment" call-to-action
// target.
func RenderStatusChangedEmail(caseID, newStatus, caseLink, commentLink string) string {
	replacer := strings.NewReplacer(
		"<!-- [CASE_ID] -->", html.EscapeString(caseID),
		"<!-- [NEW_STATUS] -->", html.EscapeString(newStatus),
		"<!-- [CASE_LINK] -->", html.EscapeString(caseLink),
		"<!-- [COMMENT_LINK] -->", html.EscapeString(commentLink),
	)
	return replacer.Replace(statusChangedTemplate)
}

// RenderCaseAssignedEmail fills in the "case assigned" HTML email template.
// assignerEmail is rendered both as a mailto: link and as plain text.
func RenderCaseAssignedEmail(assignerName, assignerEmail, caseID, caseLink, commentLink string) string {
	replacer := strings.NewReplacer(
		"<!-- [ASSIGNER_NAME] -->", html.EscapeString(assignerName),
		"<!-- [ASSIGNER_EMAIL] -->", html.EscapeString(assignerEmail),
		"<!-- [CASE_ID] -->", html.EscapeString(caseID),
		"<!-- [CASE_LINK] -->", html.EscapeString(caseLink),
		"<!-- [COMMENT_LINK] -->", html.EscapeString(commentLink),
	)
	return replacer.Replace(caseAssignedTemplate)
}

// CaseCreatedEmailData holds every value substituted into the "case created"
// HTML email template. IncidentImpactDescription is optional: when empty,
// its whole section is omitted from the output rather than rendering a
// placeholder like "null" or "N/A" for cases that don't have one (e.g.
// non-Incident case types).
type CaseCreatedEmailData struct {
	ReporterName              string
	ProjectName               string
	CaseID                    string
	CaseTitle                 string
	CaseType                  string
	Priority                  string
	Product                   string
	CreatedAt                 string
	Description               string
	IncidentImpactDescription string
	CaseLink                  string
	CommentLink               string
}

// RenderCaseCreatedEmail fills in the "case created" HTML email template.
func RenderCaseCreatedEmail(data CaseCreatedEmailData) string {
	tmpl := applyOptionalBlock(caseCreatedTemplate, "IMPACT", data.IncidentImpactDescription)
	replacer := strings.NewReplacer(
		"<!-- [REPORTER_NAME] -->", html.EscapeString(data.ReporterName),
		"<!-- [PROJECT_NAME] -->", html.EscapeString(data.ProjectName),
		"<!-- [CASE_ID] -->", html.EscapeString(data.CaseID),
		"<!-- [CASE_TITLE] -->", html.EscapeString(data.CaseTitle),
		"<!-- [CASE_TYPE] -->", html.EscapeString(data.CaseType),
		"<!-- [PRIORITY] -->", html.EscapeString(data.Priority),
		"<!-- [PRODUCT] -->", html.EscapeString(data.Product),
		"<!-- [CREATED_AT] -->", html.EscapeString(data.CreatedAt),
		"<!-- [DESCRIPTION] -->", escapeMultiline(data.Description),
		"<!-- [INCIDENT_IMPACT_DESCRIPTION] -->", escapeMultiline(data.IncidentImpactDescription),
		"<!-- [CASE_LINK] -->", html.EscapeString(data.CaseLink),
		"<!-- [COMMENT_LINK] -->", html.EscapeString(data.CommentLink),
	)
	return replacer.Replace(tmpl)
}
