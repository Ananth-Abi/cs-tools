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

package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type portalMapper func([]byte) ([]byte, error)

func mapEntityCall(result []byte, err error, mapper portalMapper) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	return mapper(result)
}

func decodePortalObject(raw []byte) (map[string]any, error) {
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("map entity response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("unexpected trailing JSON value")
		}
		return nil, fmt.Errorf("map entity response: %w", err)
	}
	return value, nil
}

func encodePortalObject(value map[string]any) ([]byte, error) {
	return json.Marshal(value)
}

func pickPortalFields(source map[string]any, fields ...string) map[string]any {
	result := make(map[string]any, len(fields))
	for _, field := range fields {
		if value, ok := source[field]; ok {
			result[field] = value
		}
	}
	return result
}

func portalStringID(value any) any {
	switch id := value.(type) {
	case json.Number:
		return id.String()
	case float64:
		return fmt.Sprintf("%g", id)
	default:
		return id
	}
}

func mapPortalReference(value any) any {
	if state, ok := value.(string); ok {
		ids := map[string]string{"ACTIVE": "2", "RESOLVED": "3", "CONVERTED": "4", "ABANDONED": "5", "CLOSED": "6"}
		return map[string]any{"id": ids[state], "label": state}
	}
	ref, ok := value.(map[string]any)
	if !ok || ref == nil {
		return nil
	}
	result := map[string]any{"id": portalStringID(ref["id"])}
	if label, ok := ref["label"]; ok {
		result["label"] = label
	} else if name, ok := ref["name"]; ok {
		result["label"] = name
	}
	return result
}

func mapPortalReferences(value any) any {
	items, ok := value.([]any)
	if !ok {
		return []any{}
	}
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, mapPortalReference(item))
	}
	return result
}

func mapPortalPagination(source, target map[string]any) {
	if total, ok := source["totalRecords"]; ok {
		target["totalRecords"] = total
	} else if total, ok := source["total"]; ok {
		target["totalRecords"] = total
	}
	for _, field := range []string{"limit", "offset"} {
		if value, ok := source[field]; ok {
			target[field] = value
		}
	}
}

func mapMetadataResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"timeZones":       mapPortalReferences(source["timeZones"]),
		"projectTypes":    mapPortalReferences(source["projectTypes"]),
		"feedbackEmojies": source["feedbackEmojies"],
		"featureFlags":    map[string]any{"usageMetricsEnabled": true},
	}
	return encodePortalObject(result)
}

func mapGlobalSearchResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	projects := make([]any, 0)
	if items, ok := source["projects"].([]any); ok {
		for _, value := range items {
			item, _ := value.(map[string]any)
			mapped := pickPortalFields(item, "id", "name", "description", "key", "createdOn", "startDate", "endDate", "hasPdpSubscription", "closureState", "activeChatsCount", "actionRequiredCount", "outstandingCount")
			mapped["type"] = mapPortalReference(item["type"])
			mapped["account"] = mapPortalReference(item["account"])
			projects = append(projects, mapped)
		}
	}
	cases := make([]any, 0)
	if items, ok := source["cases"].([]any); ok {
		for _, value := range items {
			item, _ := value.(map[string]any)
			mapped := pickPortalFields(item, "id", "internalId", "number", "title", "description", "createdOn", "createdBy", "updatedOn")
			for _, field := range []string{"project", "caseType", "state", "severity", "assignedEngineer", "account"} {
				mapped[field] = mapPortalReference(item[field])
			}
			cases = append(cases, mapped)
		}
	}
	result := pickPortalFields(source, "query", "projectsTotal", "casesTotal")
	result["projects"] = projects
	result["cases"] = cases
	return encodePortalObject(result)
}

func mapAttachmentsResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	attachments := make([]any, 0)
	if items, ok := source["attachments"].([]any); ok {
		for _, value := range items {
			item, _ := value.(map[string]any)
			mapped := pickPortalFields(item, "id", "name", "type", "createdBy", "createdOn", "downloadUrl", "previewUrl", "description")
			mapped["size"] = item["sizeBytes"]
			attachments = append(attachments, mapped)
		}
	}
	result := map[string]any{"attachments": attachments}
	mapPortalPagination(source, result)
	return encodePortalObject(result)
}

func mapCreatedAttachmentResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	attachment, _ := source["attachment"].(map[string]any)
	result := pickPortalFields(attachment, "id", "createdOn", "createdBy", "downloadUrl")
	result["size"] = attachment["sizeBytes"]
	return encodePortalObject(result)
}

func mapUpdatedAttachmentResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	attachment, _ := source["attachment"].(map[string]any)
	return encodePortalObject(pickPortalFields(attachment, "id", "name", "description", "updatedOn", "updatedBy"))
}

func mapAttachmentResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	result := pickPortalFields(source, "id", "name", "type", "createdBy", "createdOn", "downloadUrl", "previewUrl", "description")
	result["size"] = source["sizeBytes"]
	return encodePortalObject(result)
}

func mapConversationItem(item map[string]any, details bool) map[string]any {
	fields := []string{"id", "number", "initialMessage", "messageCount", "createdOn", "createdBy"}
	if details {
		fields = append(fields, "updatedOn", "updatedBy")
	}
	result := pickPortalFields(item, fields...)
	result["project"] = mapPortalReference(item["project"])
	result["case"] = mapPortalReference(item["case"])
	result["state"] = mapPortalReference(item["state"])
	return result
}

func mapConversationSearchResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	items := make([]any, 0)
	if conversations, ok := source["conversations"].([]any); ok {
		for _, value := range conversations {
			item, _ := value.(map[string]any)
			items = append(items, mapConversationItem(item, false))
		}
	}
	result := map[string]any{"conversations": items}
	mapPortalPagination(source, result)
	return encodePortalObject(result)
}

func mapConversationResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	return encodePortalObject(mapConversationItem(source, true))
}

func mapInstancesResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	items := make([]any, 0)
	if instances, ok := source["instances"].([]any); ok {
		for _, value := range instances {
			item, _ := value.(map[string]any)
			mapped := pickPortalFields(item, "id", "key", "createdOn", "updatedOn", "metadata")
			for _, field := range []string{"project", "deployedProduct", "deployment", "product"} {
				mapped[field] = mapPortalReference(item[field])
			}
			items = append(items, mapped)
		}
	}
	result := map[string]any{"instances": items}
	mapPortalPagination(source, result)
	return encodePortalObject(result)
}

func mapInstanceSeriesResponse(raw []byte, listField string) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	items := make([]any, 0)
	if series, ok := source[listField].([]any); ok {
		for _, value := range series {
			item, _ := value.(map[string]any)
			mapped := pickPortalFields(item, "instanceId", "instanceKey", "dataPoints", "periodSummaries")
			for _, field := range []string{"project", "deployment", "product", "deployedProduct"} {
				mapped[field] = mapPortalReference(item[field])
			}
			items = append(items, mapped)
		}
	}
	result := pickPortalFields(source, "totalInstances", "startDate", "endDate")
	result[listField] = items
	return encodePortalObject(result)
}

func mapInstanceMetricsResponse(raw []byte) ([]byte, error) {
	return mapInstanceSeriesResponse(raw, "metrics")
}
func mapInstanceUsageResponse(raw []byte) ([]byte, error) {
	return mapInstanceSeriesResponse(raw, "usages")
}

func mapCaseTimeCardsResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	items := make([]any, 0)
	if cases, ok := source["cases"].([]any); ok {
		for _, value := range cases {
			item, _ := value.(map[string]any)
			caseValue, _ := item["case"].(map[string]any)
			mappedCase := pickPortalFields(caseValue, "id", "number", "name", "updatedOn")
			mappedCase["project"] = mapPortalReference(caseValue["project"])
			mapped := pickPortalFields(item, "totalTime", "totalCount", "billable", "nonBillable")
			mapped["case"] = mappedCase
			items = append(items, mapped)
		}
	}
	result := map[string]any{"caseTimeCards": items}
	mapPortalPagination(source, result)
	return encodePortalObject(result)
}

func mapStatsResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	return encodePortalObject(pickPortalFields(source, "stats", "summary", "totalRecords", "startDate", "endDate"))
}

func mapEscalationItem(item map[string]any, includeUpdated bool) map[string]any {
	fields := []string{"id", "createdBy", "createdOn", "reason", "notificationSentTo"}
	if includeUpdated {
		fields = append(fields, "updatedOn")
	}
	result := pickPortalFields(item, fields...)
	for _, field := range []string{"case", "currentLevel", "previousLevel"} {
		result[field] = mapPortalReference(item[field])
	}
	return result
}

func mapCreatedEscalationResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	escalation, _ := source["escalation"].(map[string]any)
	return encodePortalObject(map[string]any{"message": source["message"], "escalation": mapEscalationItem(escalation, false)})
}

func mapEscalationsResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	items := make([]any, 0)
	if escalations, ok := source["escalations"].([]any); ok {
		for _, value := range escalations {
			item, _ := value.(map[string]any)
			items = append(items, mapEscalationItem(item, true))
		}
	}
	result := map[string]any{"escalations": items}
	mapPortalPagination(source, result)
	return encodePortalObject(result)
}

func mapDeployedProductMetricsResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	product, _ := source["deployedProduct"].(map[string]any)
	return encodePortalObject(map[string]any{
		"product":   pickPortalFields(product, "id", "name"),
		"summary":   source["summary"],
		"chartData": source["chartData"],
	})
}

func mapFeedbackResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	result := pickPortalFields(source, "id", "chips", "assessmentId", "createdBy", "createdOn", "additionalComment")
	if emoji, ok := source["emoji"].(map[string]any); ok {
		result["emoji"] = pickPortalFields(emoji, "id", "name", "selectedImage")
	}
	return encodePortalObject(result)
}

func mapSubmittedFeedbackResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	feedback, _ := source["feedback"].(map[string]any)
	return encodePortalObject(map[string]any{"message": source["message"], "feedback": pickPortalFields(feedback, "id", "emojiId", "chipIds", "assessmentId", "createdOn")})
}

func mapVulnerabilityMetadataResponse(raw []byte) ([]byte, error) {
	source, err := decodePortalObject(raw)
	if err != nil {
		return nil, err
	}
	return encodePortalObject(map[string]any{"severities": mapPortalReferences(source["severities"])})
}
