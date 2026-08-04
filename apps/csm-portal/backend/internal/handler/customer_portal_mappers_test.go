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
	"encoding/json"
	"testing"
)

func decodeMappedObject(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode mapped response: %v", err)
	}
	return result
}

func TestMapMetadataResponse_SelectsPortalFields(t *testing.T) {
	raw := []byte(`{"timeZones":[{"id":1,"label":"UTC"}],"projectTypes":[{"id":"p1","name":"Support","internal":"drop"}],"feedbackEmojies":[],"upstreamOnly":true}`)
	mapped, err := mapMetadataResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeMappedObject(t, mapped)
	if _, exists := result["upstreamOnly"]; exists {
		t.Fatal("mapper leaked an upstream-only field")
	}
	project := result["projectTypes"].([]any)[0].(map[string]any)
	if project["label"] != "Support" || project["id"] != "p1" {
		t.Fatalf("project type = %#v", project)
	}
	if result["featureFlags"].(map[string]any)["usageMetricsEnabled"] != true {
		t.Fatalf("featureFlags = %#v", result["featureFlags"])
	}
}

func TestMapMetadataResponse_PreservesLargeNumericReferenceID(t *testing.T) {
	raw := []byte(`{"timeZones":[{"id":9007199254740993,"label":"UTC"}],"projectTypes":[],"feedbackEmojies":[]}`)
	mapped, err := mapMetadataResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeMappedObject(t, mapped)
	timeZone := result["timeZones"].([]any)[0].(map[string]any)
	if timeZone["id"] != "9007199254740993" {
		t.Fatalf("time zone ID = %#v", timeZone["id"])
	}
}

func TestDecodePortalObject_RejectsTrailingJSON(t *testing.T) {
	if _, err := decodePortalObject([]byte(`{} {}`)); err == nil {
		t.Fatal("expected trailing JSON to be rejected")
	}
}

func TestMapAttachmentsResponse_RenamesSizeAndTotal(t *testing.T) {
	raw := []byte(`{"attachments":[{"id":"a1","name":"log.txt","type":"text/plain","sizeBytes":42,"createdBy":"u","createdOn":"now","downloadUrl":"/d","reference":{"id":"secret"}}],"total":1,"limit":20,"offset":0}`)
	mapped, err := mapAttachmentsResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeMappedObject(t, mapped)
	attachment := result["attachments"].([]any)[0].(map[string]any)
	if attachment["size"] != float64(42) || result["totalRecords"] != float64(1) {
		t.Fatalf("mapped response = %#v", result)
	}
	if _, exists := attachment["reference"]; exists {
		t.Fatal("mapper leaked the upstream reference")
	}
}

func TestMapInstancesResponse_MapsReferencesAndDropsExtras(t *testing.T) {
	raw := []byte(`{"instances":[{"id":"i1","key":"node","createdOn":"c","updatedOn":"u","project":{"id":"p1","name":"Project","extra":1},"metadata":{},"secret":"drop"}],"total":1,"limit":10,"offset":0}`)
	mapped, err := mapInstancesResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeMappedObject(t, mapped)
	instance := result["instances"].([]any)[0].(map[string]any)
	project := instance["project"].(map[string]any)
	if project["label"] != "Project" || result["totalRecords"] != float64(1) {
		t.Fatalf("mapped response = %#v", result)
	}
	if _, exists := instance["secret"]; exists {
		t.Fatal("mapper leaked an upstream-only instance field")
	}
}

func TestMapDeployedProductMetricsResponse_RenamesProduct(t *testing.T) {
	raw := []byte(`{"deployedProduct":{"id":"dp1","name":"API Manager","extra":true},"summary":{"totalInstances":2},"chartData":[],"debug":"drop"}`)
	mapped, err := mapDeployedProductMetricsResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeMappedObject(t, mapped)
	product := result["product"].(map[string]any)
	if product["id"] != "dp1" || product["name"] != "API Manager" {
		t.Fatalf("product = %#v", product)
	}
	if _, exists := result["deployedProduct"]; exists {
		t.Fatal("mapper retained the entity-service field name")
	}
}
