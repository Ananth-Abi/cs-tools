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

// Package aws transforms an inbound AWS notification into the canonical alert; its CloudWatch alarm mapping is a Go port of ServiceNow's AWSAlertUtilsV2.
package aws

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"sre-alert-ingestion-service/internal/model"
	"sre-alert-ingestion-service/internal/sources/jsonnum"
	"sre-alert-ingestion-service/utils"
)

// defaults mirror "edge.api.aws.alert.config" so its JSON can be pasted into AWS_ALERT_CONFIG unchanged.
var defaults = map[string]string{
	"source":      "AWS",
	"environment": "Unknown",
	"service":     "",
	"category":    "service_interruption",
	"severity":    "critical",
	"metric_name": "Unknown Metric",
}

// ErrMissingBody is returned when the webhook is called with no body, or a body that isn't valid JSON at all.
var ErrMissingBody = errors.New("missing or invalid request body")

// ErrUnsupportedType is returned for an SNS message that is neither a Notification nor one of the
// confirmations the SNS handler answers before the transform runs.
var ErrUnsupportedType = errors.New("unsupported SNS message type")

// Alert is the canonical alert model handed to the core component.
type Alert = model.Alert

// Config holds operator overrides, keyed lowercase like the ServiceNow "edge.api.aws.alert.config" property; empty fields fall back to defaults.
type Config map[string]string

// LoadConfig reads Config from the AWS_ALERT_CONFIG env var.
func LoadConfig() (Config, error) {
	raw := utils.AlertConfigRaw("AWS_ALERT_CONFIG")
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid AWS_ALERT_CONFIG: %w", err)
	}
	return cfg, nil
}

// Transform maps an SNS notification (CloudWatch alarm, EventBridge event, RDS event, Auto Scaling or any other message) or a bare EventBridge event to canonical alerts.
func Transform(raw []byte, cfg Config) ([]Alert, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return nil, ErrMissingBody
	}

	var envelope map[string]any
	if err := jsonnum.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMissingBody, err)
	}
	if t := utils.Str(envelope, "Type"); t != "" && t != "Notification" {
		return nil, fmt.Errorf("%w %q", ErrUnsupportedType, t)
	}

	base := Alert{
		Service:     configValue(cfg, "service"),
		MetricName:  configValue(cfg, "metric_name"),
		Severity:    configValue(cfg, "severity"),
		Category:    configValue(cfg, "category"),
		Environment: configValue(cfg, "environment"),
		Source:      configValue(cfg, "source"),
	}
	if isEvent(envelope) {
		return fromEvent(envelope, base), nil
	}

	// Where the notification came from, for the core's assignment-group routing; known even when the Message itself cannot be parsed.
	base.SourceTopic = utils.Str(envelope, "TopicArn")
	base.SourceAccount = arnAccount(base.SourceTopic)
	subject := utils.Str(envelope, "Subject")

	messageRaw := utils.Str(envelope, "Message")
	var message map[string]any
	if err := jsonnum.Unmarshal([]byte(messageRaw), &message); err != nil || message == nil {
		// A plain-text Message (AWS Budgets, a custom publisher) is the human-readable text itself; the raw body is kept in raw_alerts.
		base.MetricName = utils.FirstNonEmpty(subject, "SNS Message Parse Error")
		base.Description = strings.TrimSpace(messageRaw)
		return []Alert{base}, nil
	}

	switch {
	case utils.Str(message, "AlarmName") != "":
		return []Alert{fromAlarm(message, base)}, nil
	case isEvent(message):
		return fromEvent(message, base), nil
	case text(message, "Event ID") != "" && text(message, "Event Source") != "":
		return []Alert{fromRDSEvent(message, base)}, nil
	case text(message, "AutoScalingGroupName") != "" && text(message, "Event") != "":
		return []Alert{fromAutoScaling(message, base)}, nil
	}
	// Any other JSON message: its subject names it, its own message or description says what happened.
	base.MetricName = utils.FirstNonEmpty(subject, base.MetricName)
	base.Description = utils.FirstNonEmpty(text(message, "message", "Message", "description", "Description", "detail"), subject)
	return []Alert{base}, nil
}

// fromAlarm maps a CloudWatch alarm notification (metric or composite) to the canonical alert.
func fromAlarm(message map[string]any, base Alert) Alert {
	// AlarmDescription optionally carries a JSON object of field overrides; invalid JSON there degrades to no overrides, not an error.
	alarmDesc := map[string]any{}
	if adRaw := strings.TrimSpace(utils.Str(message, "AlarmDescription")); adRaw != "" {
		var parsed map[string]any
		if err := jsonnum.Unmarshal([]byte(adRaw), &parsed); err == nil && parsed != nil {
			alarmDesc = parsed
		}
	}

	// A recovered alarm (NewStateValue "OK") always forces severity "ok", overriding AlarmDescription's own override.
	if utils.Str(message, "NewStateValue") == "OK" {
		alarmDesc["severity"] = "ok"
	}

	return Alert{
		Service:          utils.FirstNonEmpty(utils.Str(alarmDesc, "service"), base.Service),
		MetricName:       utils.FirstNonEmpty(utils.Str(message, "AlarmName"), base.MetricName),
		Severity:         utils.FirstNonEmpty(utils.Str(alarmDesc, "severity"), base.Severity),
		Category:         utils.FirstNonEmpty(utils.Str(alarmDesc, "category"), base.Category),
		Environment:      utils.FirstNonEmpty(utils.Str(alarmDesc, "environment"), base.Environment),
		Source:           base.Source,
		UniqueIdentifier: utils.Str(message, "AlarmArn"),
		// The alarm's state reason, never the payload; the raw body is kept in raw_alerts.
		Description: utils.Str(message, "NewStateReason"),
		// An alarm may name its own CSM assignment group in AlarmDescription; it beats every other routing signal the core has.
		AssignmentGroup: strings.TrimSpace(utils.Str(alarmDesc, "assignment_group")),
		SourceTopic:     base.SourceTopic,
		SourceAccount: utils.FirstNonEmpty(
			utils.Str(message, "AWSAccountId"),
			arnAccount(utils.Str(message, "AlarmArn")),
			base.SourceAccount,
		),
	}
}

// arnAccount returns the account id of an ARN (arn:partition:service:region:account:resource), or "".
func arnAccount(arn string) string {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 6 || parts[0] != "arn" {
		return ""
	}
	return parts[4]
}

// configValue applies the 2-tier resolution: operator config, then the hardcoded default.
func configValue(cfg Config, field string) string {
	return utils.FirstNonEmpty(cfg[field], defaults[field])
}
