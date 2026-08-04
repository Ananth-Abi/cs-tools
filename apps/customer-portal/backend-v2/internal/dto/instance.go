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

// Package dto's instance-related types and mappers back the 15 portal
// endpoints fanned out from entity-service's 5 instance endpoints: each of
// searchInstances/searchInstanceMetrics/searchInstanceUsage/
// searchInstanceMetricsStats/searchInstanceUsageStats is exposed as three
// portal routes (project-scoped, deployment-scoped, deployed-product-scoped)
// that each force exactly one ID filter from the URL path, matching the
// Ballerina reference's own fan-out. The mapping logic itself is identical
// across all three scopes — only the handler layer differs in which path
// param feeds which entity filter field.
package dto

import "github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"

func toOptionalRef(r *entity.ReferenceTableItem) *ReferenceItem {
	if r == nil {
		return nil
	}
	return &ReferenceItem{ID: r.ID, Label: r.Name}
}

// InstanceSearchRequest is the portal's request body for the three
// instances/search routes. Exactly one path param (project/deployment/
// deployed-product ID) is injected server-side into the corresponding
// entity filter field by the calling handler.
type InstanceSearchRequest struct {
	StartDate  *string           `json:"startDate,omitempty"`
	EndDate    *string           `json:"endDate,omitempty"`
	Pagination entity.Pagination `json:"pagination"`
}

// Instance is a single instance row.
type Instance struct {
	ID              string                   `json:"id"`
	Key             string                   `json:"key"`
	Project         *ReferenceItem           `json:"project"`
	Deployment      *ReferenceItem           `json:"deployment"`
	Product         *ReferenceItem           `json:"product"`
	DeployedProduct *ReferenceItem           `json:"deployedProduct"`
	CreatedOn       string                   `json:"createdOn"`
	UpdatedOn       string                   `json:"updatedOn"`
	Metadata        *entity.InstanceMetadata `json:"metadata"`
}

// InstanceSearchResponse is the portal's response for the three instances/search routes.
type InstanceSearchResponse struct {
	Instances []Instance `json:"instances"`
	Total     int        `json:"total"`
	Offset    int        `json:"offset"`
	Limit     int        `json:"limit"`
}

// MapInstanceSearchResponse builds the portal response from entity-service's
// SearchInstancesResponse, matching the Ballerina reference's mapInstancesResponse.
func MapInstanceSearchResponse(r entity.SearchInstancesResponse) InstanceSearchResponse {
	items := make([]Instance, 0, len(r.Instances))
	for _, i := range r.Instances {
		items = append(items, Instance{
			ID:              i.ID,
			Key:             i.Key,
			Project:         toOptionalRef(i.Project),
			Deployment:      toOptionalRef(i.Deployment),
			Product:         toOptionalRef(i.Product),
			DeployedProduct: toOptionalRef(i.DeployedProduct),
			CreatedOn:       i.CreatedOn,
			UpdatedOn:       i.UpdatedOn,
			Metadata:        i.Metadata,
		})
	}
	return InstanceSearchResponse{Instances: items, Total: r.Total, Offset: r.Offset, Limit: r.Limit}
}

// InstanceDateRangeRequest is the portal's request body for the
// instances/metrics/search and instances/usages/search route families.
// StartDate/EndDate are required; the path-scoped ID filter is injected
// server-side by the calling handler.
type InstanceDateRangeRequest struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// InstanceMetric is one instance's metric time series, ordered newest to oldest.
type InstanceMetric struct {
	InstanceID      string                     `json:"instanceId"`
	InstanceKey     string                     `json:"instanceKey"`
	Project         *ReferenceItem             `json:"project"`
	Deployment      *ReferenceItem             `json:"deployment"`
	Product         *ReferenceItem             `json:"product"`
	DeployedProduct *ReferenceItem             `json:"deployedProduct"`
	DataPoints      []entity.InstanceDataPoint `json:"dataPoints"`
}

// InstanceMetricsResponse is the portal's response for the instances/metrics/search routes.
type InstanceMetricsResponse struct {
	Metrics        []InstanceMetric `json:"metrics"`
	TotalInstances int              `json:"totalInstances"`
	StartDate      string           `json:"startDate"`
	EndDate        string           `json:"endDate"`
}

// MapInstanceMetricsResponse builds the portal response from entity-service's
// InstanceMetricsResponse, matching the Ballerina reference's mapInstanceMetrics.
func MapInstanceMetricsResponse(r entity.InstanceMetricsResponse) InstanceMetricsResponse {
	metrics := make([]InstanceMetric, 0, len(r.Metrics))
	for _, m := range r.Metrics {
		metrics = append(metrics, InstanceMetric{
			InstanceID:      m.InstanceID,
			InstanceKey:     m.InstanceKey,
			Project:         toOptionalRef(m.Project),
			Deployment:      toOptionalRef(m.Deployment),
			Product:         toOptionalRef(m.Product),
			DeployedProduct: toOptionalRef(m.DeployedProduct),
			DataPoints:      m.DataPoints,
		})
	}
	return InstanceMetricsResponse{
		Metrics:        metrics,
		TotalInstances: r.TotalInstances,
		StartDate:      r.StartDate,
		EndDate:        r.EndDate,
	}
}

// InstanceUsageEntry is one instance's usage time series.
type InstanceUsageEntry struct {
	InstanceID      string                   `json:"instanceId"`
	InstanceKey     string                   `json:"instanceKey"`
	Project         *ReferenceItem           `json:"project"`
	Deployment      *ReferenceItem           `json:"deployment"`
	Product         *ReferenceItem           `json:"product"`
	DeployedProduct *ReferenceItem           `json:"deployedProduct"`
	PeriodSummaries []entity.InstanceSummary `json:"periodSummaries"`
}

// InstanceUsageResponse is the portal's response for the instances/usages/search routes.
type InstanceUsageResponse struct {
	Usages         []InstanceUsageEntry `json:"usages"`
	TotalInstances int                  `json:"totalInstances"`
	StartDate      string               `json:"startDate"`
	EndDate        string               `json:"endDate"`
}

// MapInstanceUsageResponse builds the portal response from entity-service's
// InstanceUsageResponse, matching the Ballerina reference's mapInstanceUsages.
func MapInstanceUsageResponse(r entity.InstanceUsageResponse) InstanceUsageResponse {
	usages := make([]InstanceUsageEntry, 0, len(r.Usages))
	for _, u := range r.Usages {
		usages = append(usages, InstanceUsageEntry{
			InstanceID:      u.InstanceID,
			InstanceKey:     u.InstanceKey,
			Project:         toOptionalRef(u.Project),
			Deployment:      toOptionalRef(u.Deployment),
			Product:         toOptionalRef(u.Product),
			DeployedProduct: toOptionalRef(u.DeployedProduct),
			PeriodSummaries: u.PeriodSummaries,
		})
	}
	return InstanceUsageResponse{
		Usages:         usages,
		TotalInstances: r.TotalInstances,
		StartDate:      r.StartDate,
		EndDate:        r.EndDate,
	}
}

// InstanceStatsRequest is the portal's request body for the
// instances/stats/metrics/search and instances/stats/usages/search route
// families. StartDate/EndDate are required; DataSource is optional (1 = API
// Call, 2 = File Upload). Note: the Ballerina reference's project-scoped
// metrics-stats variant does NOT forward DataSource to entity-service (a
// real asymmetry, not a bug) — see InstanceMetricsStatsRequestFilters's doc
// comment on the handler side for how this is preserved.
type InstanceStatsRequest struct {
	StartDate  string `json:"startDate"`
	EndDate    string `json:"endDate"`
	DataSource *int   `json:"dataSource,omitempty"`
}

// InstanceMetricSummary is the current/min/max/avg summary for a metrics-stats query.
type InstanceMetricSummary struct {
	Current float64 `json:"current"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Avg     float64 `json:"avg"`
}

// InstanceMetricsStatsResponse is the portal's response for the
// instances/stats/metrics/search routes.
type InstanceMetricsStatsResponse struct {
	Stats     map[string]map[string]int `json:"stats"`
	Summary   InstanceMetricSummary     `json:"summary"`
	Total     int                       `json:"total"`
	StartDate string                    `json:"startDate"`
	EndDate   string                    `json:"endDate"`
}

// MapInstanceMetricsStatsResponse builds the portal response from
// entity-service's InstanceMetricsStatsResponse, matching the Ballerina
// reference's mapInstanceMetricStats.
func MapInstanceMetricsStatsResponse(r entity.InstanceMetricsStatsResponse) InstanceMetricsStatsResponse {
	return InstanceMetricsStatsResponse{
		Stats: r.Stats,
		Summary: InstanceMetricSummary{
			Current: r.Summary.Current,
			Min:     r.Summary.Min,
			Max:     r.Summary.Max,
			Avg:     r.Summary.Avg,
		},
		Total:     r.Total,
		StartDate: r.StartDate,
		EndDate:   r.EndDate,
	}
}

// InstanceUsageStatsResponse is the portal's response for the
// instances/stats/usages/search routes. Unlike InstanceMetricsStatsResponse,
// there is no summary block, matching the Ballerina reference exactly.
type InstanceUsageStatsResponse struct {
	Stats     map[string]map[string]int `json:"stats"`
	Total     int                       `json:"total"`
	StartDate string                    `json:"startDate"`
	EndDate   string                    `json:"endDate"`
}

// MapInstanceUsageStatsResponse builds the portal response from
// entity-service's InstanceUsageStatsResponse, matching the Ballerina
// reference's mapInstanceUsageStats.
func MapInstanceUsageStatsResponse(r entity.InstanceUsageStatsResponse) InstanceUsageStatsResponse {
	return InstanceUsageStatsResponse{
		Stats:     r.Stats,
		Total:     r.Total,
		StartDate: r.StartDate,
		EndDate:   r.EndDate,
	}
}
