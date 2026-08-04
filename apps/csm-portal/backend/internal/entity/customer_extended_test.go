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

package entity

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCustomerEntityClient_ExtendedEndpointContracts(t *testing.T) {
	t.Parallel()

	type invocation struct {
		method string
		path   string
		call   func(*CustomerEntityClient) error
	}
	body := []byte(`{"filters":{}}`)
	id := "11111111-1111-1111-1111-111111111111"
	tests := []invocation{
		{http.MethodGet, "/metadata", func(c *CustomerEntityClient) error { _, err := c.GetMetadata(context.Background()); return err }},
		{http.MethodPost, "/search", func(c *CustomerEntityClient) error { _, err := c.GlobalSearch(context.Background(), body); return err }},
		{http.MethodPost, "/instances/search", func(c *CustomerEntityClient) error {
			_, err := c.SearchInstances(context.Background(), body)
			return err
		}},
		{http.MethodGet, "/attachments/" + id, func(c *CustomerEntityClient) error { _, err := c.GetAttachment(context.Background(), id); return err }},
		{http.MethodPatch, "/attachments/" + id, func(c *CustomerEntityClient) error {
			_, err := c.PatchAttachment(context.Background(), id, body)
			return err
		}},
		{http.MethodGet, "/cases/" + id + "/feedback", func(c *CustomerEntityClient) error { _, err := c.GetCaseFeedback(context.Background(), id); return err }},
		{http.MethodPost, "/cases/" + id + "/feedback", func(c *CustomerEntityClient) error {
			_, err := c.SubmitCaseFeedback(context.Background(), id, body)
			return err
		}},
		{http.MethodGet, "/conversations/" + id, func(c *CustomerEntityClient) error { _, err := c.GetConversation(context.Background(), id); return err }},
		{http.MethodPost, "/conversations", func(c *CustomerEntityClient) error {
			_, err := c.CreateConversation(context.Background(), body)
			return err
		}},
		{http.MethodPatch, "/conversations/" + id, func(c *CustomerEntityClient) error {
			_, err := c.UpdateConversation(context.Background(), id, body)
			return err
		}},
		{http.MethodGet, "/products/vulnerabilities/meta", func(c *CustomerEntityClient) error {
			_, err := c.GetProductVulnerabilityMetadata(context.Background())
			return err
		}},
		{http.MethodPost, "/cases/time-cards/search", func(c *CustomerEntityClient) error {
			_, err := c.SearchCaseTimeCards(context.Background(), body)
			return err
		}},
		{http.MethodPost, "/instances/metrics/search", func(c *CustomerEntityClient) error {
			_, err := c.SearchInstanceMetrics(context.Background(), body)
			return err
		}},
		{http.MethodPost, "/instances/usages/search", func(c *CustomerEntityClient) error {
			_, err := c.SearchInstanceUsage(context.Background(), body)
			return err
		}},
		{http.MethodPost, "/instances/metrics/stats/search", func(c *CustomerEntityClient) error {
			_, err := c.SearchInstanceMetricsStats(context.Background(), body)
			return err
		}},
		{http.MethodPost, "/instances/usages/stats/search", func(c *CustomerEntityClient) error {
			_, err := c.SearchInstanceUsageStats(context.Background(), body)
			return err
		}},
		{http.MethodPost, "/escalations", func(c *CustomerEntityClient) error {
			_, err := c.CreateEscalation(context.Background(), body)
			return err
		}},
		{http.MethodPost, "/escalations/search", func(c *CustomerEntityClient) error {
			_, err := c.SearchEscalations(context.Background(), body)
			return err
		}},
		{http.MethodPost, "/deployed-products/" + id + "/metrics/search", func(c *CustomerEntityClient) error {
			_, err := c.SearchDeployedProductMetrics(context.Background(), id, body)
			return err
		}},
		{http.MethodPost, "/deployed-products/" + id + "/metrics/usage-counts/search", func(c *CustomerEntityClient) error {
			_, err := c.SearchDeployedProductUsageCounts(context.Background(), id, body)
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method || r.URL.EscapedPath() != tc.path {
					t.Errorf("request = %s %s, want %s %s", r.Method, r.URL.EscapedPath(), tc.method, tc.path)
				}
				if tc.method == http.MethodPost || tc.method == http.MethodPatch {
					got, _ := io.ReadAll(r.Body)
					if string(got) != string(body) {
						t.Errorf("body = %s, want %s", got, body)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()
			client := &CustomerEntityClient{http: server.Client(), baseURL: server.URL}
			if err := tc.call(client); err != nil {
				t.Fatalf("call failed: %v", err)
			}
		})
	}
}
