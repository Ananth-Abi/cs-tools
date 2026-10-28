// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package aws

import (
	"fmt"
	"strconv"
	"strings"

	"sre-alert-ingestion-service/utils"
)

// securityCategory files a finding as a CSM SECURITY incident.
const securityCategory = "security"

// isEvent reports whether m is an EventBridge event.
func isEvent(m map[string]any) bool {
	return text(m, "detail-type") != "" && text(m, "source") != ""
}

// fromEvent maps an EventBridge event; a Security Hub event yields one alert per finding.
func fromEvent(ev map[string]any, base Alert) []Alert {
	detail := obj(ev, "detail")
	base.SourceAccount = utils.FirstNonEmpty(text(ev, "account"), base.SourceAccount)
	switch text(ev, "detail-type") {
	case "CloudWatch Alarm State Change":
		return []Alert{fromAlarm(alarmFromEvent(ev, detail), base)}
	case "GuardDuty Finding":
		return []Alert{fromGuardDuty(detail, base)}
	case "AWS Health Event":
		return []Alert{fromHealth(detail, base)}
	case "Security Hub Findings - Imported":
		if alerts := fromSecurityHub(detail, base); len(alerts) > 0 {
			return alerts
		}
	}
	return []Alert{fromGenericEvent(ev, detail, base)}
}

// alarmFromEvent reshapes a "CloudWatch Alarm State Change" event into the SNS alarm message, so both deliveries of one alarm share a fingerprint.
func alarmFromEvent(ev, detail map[string]any) map[string]any {
	state := obj(detail, "state")
	return map[string]any{
		"AlarmName":        text(detail, "alarmName"),
		"AlarmArn":         first(strs(ev, "resources")),
		"NewStateValue":    text(state, "value"),
		"NewStateReason":   text(state, "reason"),
		"AlarmDescription": text(obj(detail, "configuration"), "description"),
		"AWSAccountId":     text(ev, "account"),
	}
}

// fromGuardDuty maps a GuardDuty finding: its type names it and its id keeps repeat sightings on one incident.
func fromGuardDuty(detail map[string]any, base Alert) Alert {
	base.MetricName = utils.FirstNonEmpty(text(detail, "type"), "GuardDuty Finding")
	base.UniqueIdentifier = utils.FirstNonEmpty(text(detail, "id"), text(detail, "arn"))
	base.Description = utils.FirstNonEmpty(text(detail, "description"), text(detail, "title"))
	base.Category = securityCategory
	base.SourceAccount = utils.FirstNonEmpty(text(detail, "accountId"), base.SourceAccount)
	if sev, err := strconv.ParseFloat(text(detail, "severity"), 64); err == nil {
		base.Severity = guardDutySeverity(sev)
	}
	return base
}

// guardDutySeverity maps GuardDuty's 0-10 score: critical from 9, high from 7, medium from 4, low below.
func guardDutySeverity(score float64) string {
	switch {
	case score >= 9:
		return "critical"
	case score >= 7:
		return "major"
	case score >= 4:
		return "minor"
	}
	return "warning"
}

// fromHealth maps an AWS Health event; its closed status resolves the incident its open one raised.
func fromHealth(detail map[string]any, base Alert) Alert {
	base.MetricName = utils.FirstNonEmpty(text(detail, "eventTypeCode"), "AWS Health Event")
	base.UniqueIdentifier = text(detail, "eventArn")
	if descs := list(detail, "eventDescription"); len(descs) > 0 {
		base.Description = text(descs[0], "latestDescription")
	}
	switch {
	case strings.EqualFold(text(detail, "statusCode"), "closed"):
		base.Severity = "ok"
	case !strings.EqualFold(text(detail, "eventTypeCategory"), "issue"):
		// A scheduled change or an account notification is notice, not an outage.
		base.Severity = "warning"
	}
	return base
}

// fromSecurityHub maps each finding of a Security Hub event; a resolved, suppressed or archived finding resolves its incident.
func fromSecurityHub(detail map[string]any, base Alert) []Alert {
	var out []Alert
	for _, f := range list(detail, "findings") {
		a := base
		a.MetricName = utils.FirstNonEmpty(text(f, "Title"), "Security Hub Finding")
		a.UniqueIdentifier = text(f, "Id")
		a.Description = text(f, "Description")
		a.Category = securityCategory
		a.SourceAccount = utils.FirstNonEmpty(text(f, "AwsAccountId"), base.SourceAccount)
		if sev := securityHubSeverity(text(obj(f, "Severity"), "Label")); sev != "" {
			a.Severity = sev
		}
		switch strings.ToUpper(text(obj(f, "Workflow"), "Status")) {
		case "RESOLVED", "SUPPRESSED":
			a.Severity = "ok"
		}
		if strings.EqualFold(text(f, "RecordState"), "ARCHIVED") {
			a.Severity = "ok"
		}
		out = append(out, a)
	}
	return out
}

// securityHubSeverity maps Security Hub's severity label, or "" for an unknown one.
func securityHubSeverity(label string) string {
	switch strings.ToUpper(label) {
	case "CRITICAL":
		return "critical"
	case "HIGH":
		return "major"
	case "MEDIUM":
		return "minor"
	case "LOW", "INFORMATIONAL":
		return "warning"
	}
	return ""
}

// fromGenericEvent maps any other EventBridge event: its detail-type names it and its resources identify it.
func fromGenericEvent(ev, detail map[string]any, base Alert) Alert {
	resources := strs(ev, "resources")
	base.MetricName = text(ev, "detail-type")
	base.UniqueIdentifier = utils.FirstNonEmpty(strings.Join(resources, ","), text(ev, "id"))
	summary := fmt.Sprintf("%s from %s", base.MetricName, text(ev, "source"))
	if region := text(ev, "region"); region != "" {
		summary += " in " + region
	}
	if len(resources) > 0 {
		summary += ": " + strings.Join(resources, ", ")
	}
	base.Description = utils.FirstNonEmpty(text(detail, "message", "description", "reason", "status-message"), summary)
	return base
}
