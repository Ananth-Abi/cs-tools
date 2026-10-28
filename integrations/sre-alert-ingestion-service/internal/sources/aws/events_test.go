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
	"encoding/json"
	"testing"
)

// transformOne is Transform for a notification that yields exactly one alert.
func transformOne(raw []byte, cfg Config) (Alert, error) {
	alerts, err := Transform(raw, cfg)
	if err != nil || len(alerts) == 0 {
		return Alert{}, err
	}
	return alerts[0], nil
}

func event(detailType, source string, detail any, resources ...string) map[string]any {
	return map[string]any{
		"version": "0", "id": "ev-1", "detail-type": detailType, "source": source,
		"account": "111122223333", "region": "us-east-1", "resources": resources, "detail": detail,
	}
}

func mustOne(t *testing.T, raw []byte) Alert {
	t.Helper()
	alerts, err := Transform(raw, Config{})
	if err != nil || len(alerts) != 1 {
		t.Fatalf("Transform = %+v, %v; want one alert", alerts, err)
	}
	return alerts[0]
}

// One alarm delivered by SNS and by EventBridge is the same alert, so both land on one incident.
func TestTransform_AlarmStateChangeEventMatchesSNSAlarm(t *testing.T) {
	arn := "arn:aws:cloudwatch:us-east-1:123456789012:alarm:HighCPUAlarm"
	ev := event("CloudWatch Alarm State Change", "aws.cloudwatch", map[string]any{
		"alarmName":     "HighCPUAlarm",
		"state":         map[string]any{"value": "ALARM", "reason": "Threshold Crossed"},
		"configuration": map[string]any{"description": `{"service":"choreo","severity":"major"}`},
	}, arn)
	got := mustOne(t, snsEnvelope(ev))
	if got.MetricName != "HighCPUAlarm" || got.UniqueIdentifier != arn || got.Service != "choreo" || got.Severity != "major" || got.Description != "Threshold Crossed" || got.SourceAccount != "111122223333" {
		t.Errorf("alert = %+v", got)
	}

	ev["detail"].(map[string]any)["state"] = map[string]any{"value": "OK"}
	if a := mustOne(t, snsEnvelope(ev)); a.Severity != "ok" || a.UniqueIdentifier != arn {
		t.Errorf("recovered alert = %+v; want severity ok on the same alarm", a)
	}
}

// An EventBridge API destination posts the event itself, with no SNS envelope.
func TestTransform_BareEventBridgeEvent(t *testing.T) {
	raw, _ := json.Marshal(event("GuardDuty Finding", "aws.guardduty", map[string]any{
		"id": "f-1", "type": "Recon:EC2/PortProbeUnprotectedPort", "severity": 8.0, "accountId": "444455556666",
		"title": "Port probed", "description": "EC2 instance i-1 has an unprotected port being probed.",
	}))
	a := mustOne(t, raw)
	if a.MetricName != "Recon:EC2/PortProbeUnprotectedPort" || a.UniqueIdentifier != "f-1" || a.Severity != "major" || a.Category != "security" ||
		a.Description != "EC2 instance i-1 has an unprotected port being probed." || a.SourceAccount != "444455556666" || a.SourceTopic != "" {
		t.Errorf("alert = %+v", a)
	}
}

func TestGuardDutySeverity(t *testing.T) {
	for score, want := range map[float64]string{9.5: "critical", 9: "critical", 7: "major", 5.5: "minor", 2: "warning"} {
		if got := guardDutySeverity(score); got != want {
			t.Errorf("guardDutySeverity(%v) = %q, want %q", score, got, want)
		}
	}
}

// A Health issue raises at the configured severity, its closing resolves it, and a scheduled change is only a warning.
func TestTransform_HealthEvent(t *testing.T) {
	detail := map[string]any{
		"eventArn": "arn:aws:health:us-east-1::event/EC2/AWS_EC2_OPERATIONAL_ISSUE/1", "eventTypeCode": "AWS_EC2_OPERATIONAL_ISSUE",
		"eventTypeCategory": "issue", "statusCode": "open",
		"eventDescription": []any{map[string]any{"language": "en_US", "latestDescription": "Increased API error rates."}},
	}
	a := mustOne(t, snsEnvelope(event("AWS Health Event", "aws.health", detail)))
	if a.MetricName != "AWS_EC2_OPERATIONAL_ISSUE" || a.Severity != "critical" || a.Description != "Increased API error rates." || a.UniqueIdentifier == "" {
		t.Errorf("open issue = %+v", a)
	}
	detail["statusCode"] = "closed"
	if a := mustOne(t, snsEnvelope(event("AWS Health Event", "aws.health", detail))); a.Severity != "ok" {
		t.Errorf("closed issue severity = %q, want ok", a.Severity)
	}
	detail["statusCode"], detail["eventTypeCategory"] = "upcoming", "scheduledChange"
	if a := mustOne(t, snsEnvelope(event("AWS Health Event", "aws.health", detail))); a.Severity != "warning" {
		t.Errorf("scheduled change severity = %q, want warning", a.Severity)
	}
}

// Each Security Hub finding is its own alert; a resolved one resolves its incident.
func TestTransform_SecurityHubFindings(t *testing.T) {
	ev := event("Security Hub Findings - Imported", "aws.securityhub", map[string]any{"findings": []any{
		map[string]any{"Id": "f-1", "Title": "S3.8 bucket allows public access", "Description": "Block public access is off.", "AwsAccountId": "777788889999", "Severity": map[string]any{"Label": "HIGH"}, "Workflow": map[string]any{"Status": "NEW"}, "RecordState": "ACTIVE"},
		map[string]any{"Id": "f-2", "Title": "IAM.1 policy allows full admin", "Severity": map[string]any{"Label": "CRITICAL"}, "Workflow": map[string]any{"Status": "RESOLVED"}},
	}})
	alerts, err := Transform(snsEnvelope(ev), Config{})
	if err != nil || len(alerts) != 2 {
		t.Fatalf("Transform = %+v, %v; want two alerts", alerts, err)
	}
	if a := alerts[0]; a.UniqueIdentifier != "f-1" || a.Severity != "major" || a.Category != "security" || a.SourceAccount != "777788889999" || a.MetricName != "S3.8 bucket allows public access" {
		t.Errorf("first finding = %+v", a)
	}
	if a := alerts[1]; a.UniqueIdentifier != "f-2" || a.Severity != "ok" {
		t.Errorf("resolved finding = %+v", a)
	}
}

// Any other event is named by its detail-type and identified by its resources.
func TestTransform_OtherEventBridgeEvent(t *testing.T) {
	arn := "arn:aws:ec2:us-east-1:111122223333:instance/i-1"
	a := mustOne(t, snsEnvelope(event("EC2 Instance State-change Notification", "aws.ec2", map[string]any{"instance-id": "i-1", "state": "stopped"}, arn)))
	if a.MetricName != "EC2 Instance State-change Notification" || a.UniqueIdentifier != arn || a.Severity != "critical" || a.SourceAccount != "111122223333" ||
		a.Description != "EC2 Instance State-change Notification from aws.ec2 in us-east-1: "+arn {
		t.Errorf("alert = %+v", a)
	}
}

func TestTransform_RDSEvent(t *testing.T) {
	a := mustOne(t, snsEnvelope(map[string]any{
		"Event Source": "db-instance", "Source ID": "orders-db", "Source ARN": "arn:aws:rds:us-east-1:222233334444:db:orders-db",
		"Event ID":      "http://docs.amazonwebservices.com/AmazonRDS/latest/UserGuide/USER_Events.html#RDS-EVENT-0031",
		"Event Message": "DB instance failed",
	}))
	if a.MetricName != "RDS-EVENT-0031" || a.UniqueIdentifier != "arn:aws:rds:us-east-1:222233334444:db:orders-db" || a.Description != "DB instance failed" || a.SourceAccount != "222233334444" {
		t.Errorf("alert = %+v", a)
	}
}

// A failed launch keeps the configured severity; a successful one is only a warning.
func TestTransform_AutoScaling(t *testing.T) {
	msg := map[string]any{
		"Event": "autoscaling:EC2_INSTANCE_LAUNCH_ERROR", "AccountId": "333344445555", "AutoScalingGroupName": "web",
		"AutoScalingGroupARN": "arn:aws:autoscaling:us-east-1:333344445555:autoScalingGroup:x:autoScalingGroupName/web",
		"Description":         "Launching a new EC2 instance. Status Reason: capacity", "Cause": "At 2026-10-28 an instance was started.",
	}
	a := mustOne(t, snsEnvelope(msg))
	if a.MetricName != "autoscaling:EC2_INSTANCE_LAUNCH_ERROR" || a.Severity != "critical" || a.SourceAccount != "333344445555" ||
		a.Description != "Launching a new EC2 instance. Status Reason: capacity\nAt 2026-10-28 an instance was started." {
		t.Errorf("launch error = %+v", a)
	}
	msg["Event"] = "autoscaling:EC2_INSTANCE_LAUNCH"
	if a := mustOne(t, snsEnvelope(msg)); a.Severity != "warning" {
		t.Errorf("launch severity = %q, want warning", a.Severity)
	}
}

// Any other message is named by its SNS Subject rather than "Unknown Metric" or "SNS Message Parse Error".
func TestTransform_OtherMessagesUseTheSubject(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"Type": "Notification", "Subject": "AWS Budgets: monthly has exceeded your alert threshold", "Message": "You have exceeded 80% of your budget."})
	if a := mustOne(t, raw); a.MetricName != "AWS Budgets: monthly has exceeded your alert threshold" || a.Description != "You have exceeded 80% of your budget." {
		t.Errorf("text message = %+v", a)
	}
	raw, _ = json.Marshal(map[string]any{"Type": "Notification", "Subject": "Backup failed", "Message": `{"message":"Backup job b-1 failed","state":"FAILED"}`})
	if a := mustOne(t, raw); a.MetricName != "Backup failed" || a.Description != "Backup job b-1 failed" {
		t.Errorf("JSON message = %+v", a)
	}
	raw, _ = json.Marshal(map[string]any{"Type": "Notification", "Message": "no subject here"})
	if a := mustOne(t, raw); a.MetricName != "SNS Message Parse Error" {
		t.Errorf("no subject metric = %q, want the old fallback", a.MetricName)
	}
}
