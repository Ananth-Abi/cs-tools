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
	"strings"

	"sre-alert-ingestion-service/utils"
)

// fromRDSEvent maps an RDS event subscription notification: its RDS-EVENT code names it and its source identifies it.
func fromRDSEvent(message map[string]any, base Alert) Alert {
	eventID := text(message, "Event ID")
	if _, code, ok := strings.Cut(eventID, "#"); ok {
		eventID = code
	}
	base.MetricName = eventID
	base.UniqueIdentifier = utils.FirstNonEmpty(text(message, "Source ARN"), text(message, "Source ID"))
	base.Description = text(message, "Event Message")
	base.SourceAccount = utils.FirstNonEmpty(arnAccount(text(message, "Source ARN")), base.SourceAccount)
	return base
}

// fromAutoScaling maps an Auto Scaling notification; only a failed launch or termination keeps the configured severity.
func fromAutoScaling(message map[string]any, base Alert) Alert {
	event := text(message, "Event")
	base.MetricName = event
	base.UniqueIdentifier = utils.FirstNonEmpty(text(message, "AutoScalingGroupARN"), text(message, "AutoScalingGroupName"))
	base.Description = strings.TrimSpace(strings.Join(nonEmpty(text(message, "Description"), text(message, "Cause"), text(message, "StatusMessage")), "\n"))
	base.SourceAccount = utils.FirstNonEmpty(text(message, "AccountId"), arnAccount(text(message, "AutoScalingGroupARN")), base.SourceAccount)
	if !strings.HasSuffix(event, "_ERROR") {
		base.Severity = "warning"
	}
	return base
}

// text returns the first of keys whose value is a non-empty string or number; nested objects and arrays are skipped.
func text(m map[string]any, keys ...string) string {
	for _, k := range keys {
		switch v := m[k].(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		case json.Number:
			return v.String()
		case float64, bool:
			return utils.Str(m, k)
		}
	}
	return ""
}

// obj returns m[key] as an object, or nil.
func obj(m map[string]any, key string) map[string]any {
	o, _ := m[key].(map[string]any)
	return o
}

// list returns the objects in the array m[key].
func list(m map[string]any, key string) []map[string]any {
	items, _ := m[key].([]any)
	var out []map[string]any
	for _, it := range items {
		if o, ok := it.(map[string]any); ok {
			out = append(out, o)
		}
	}
	return out
}

// strs returns the non-empty strings in the array m[key].
func strs(m map[string]any, key string) []string {
	items, _ := m[key].([]any)
	var out []string
	for _, it := range items {
		if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// first returns the first element, or "".
func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// nonEmpty drops the empty values.
func nonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
