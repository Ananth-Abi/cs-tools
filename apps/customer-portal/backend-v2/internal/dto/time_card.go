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

package dto

import "github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"

// TimeCardCaseRef is a reference to the case associated with a time card.
type TimeCardCaseRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Number string `json:"number"`
}

// TimeCardSummary is one item of the portal's response for POST /time-cards/search.
//
// Deliberately excludes entity-service's per-category time breakdowns
// (timeAnalyzing, timeSettingUp, timeReproducingDebugging,
// timeProvidingSolution, timePatching), issueComplexity, workLogComment,
// rejectionReason, and the eligible-approvers list — internal WSO2 support
// bookkeeping the Ballerina backend this is rewriting never exposed to the
// customer portal either.
type TimeCardSummary struct {
	ID          string           `json:"id"`
	TotalTime   float64          `json:"totalTime"`
	WorkDate    string           `json:"workDate"`
	HasBillable bool             `json:"hasBillable"`
	State       *string          `json:"state,omitempty"`
	User        *Ref             `json:"user,omitempty"`
	ApprovedBy  *Ref             `json:"approvedBy,omitempty"`
	Project     *Ref             `json:"project,omitempty"`
	Case        *TimeCardCaseRef `json:"case,omitempty"`
}

// SearchTimeCardsResponse is the portal's response for POST /time-cards/search.
type SearchTimeCardsResponse struct {
	TimeCards []TimeCardSummary `json:"timeCards"`
	Total     int               `json:"total"`
	Limit     int               `json:"limit"`
	Offset    int               `json:"offset"`
}

// MapSearchTimeCards builds the portal response from entity-service's SearchTimeCardsResponse.
func MapSearchTimeCards(r entity.SearchTimeCardsResponse) SearchTimeCardsResponse {
	items := make([]TimeCardSummary, 0, len(r.TimeCards))
	for _, t := range r.TimeCards {
		items = append(items, TimeCardSummary{
			ID:          t.ID,
			TotalTime:   t.TotalTime,
			WorkDate:    t.WorkDate,
			HasBillable: t.HasBillable,
			State:       t.State,
			User:        mapTimeCardRef(t.User),
			ApprovedBy:  mapTimeCardRef(t.ApprovedBy),
			Project:     mapTimeCardRef(t.Project),
			Case:        mapTimeCardCaseRef(t.Case),
		})
	}
	return SearchTimeCardsResponse{
		TimeCards: items,
		Total:     r.Total,
		Limit:     r.Limit,
		Offset:    r.Offset,
	}
}

func mapTimeCardRef(r *entity.TimeCardRef) *Ref {
	if r == nil {
		return nil
	}
	return &Ref{ID: r.ID, Name: r.Name}
}

func mapTimeCardCaseRef(r *entity.TimeCardCaseRef) *TimeCardCaseRef {
	if r == nil {
		return nil
	}
	return &TimeCardCaseRef{ID: r.ID, Name: r.Name, Number: r.Number}
}
