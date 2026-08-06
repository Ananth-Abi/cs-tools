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

// CallRequestCreateResponse is the portal's response for POST /call-requests.
type CallRequestCreateResponse struct {
	ID        string `json:"id"`
	CreatedOn string `json:"createdOn"`
	State     string `json:"state"`
}

// MapCallRequestCreate builds the portal response from entity-service's CreateCallRequestResponse.
func MapCallRequestCreate(r entity.CreateCallRequestResponse) CallRequestCreateResponse {
	return CallRequestCreateResponse{
		ID:        r.CallRequest.ID,
		CreatedOn: r.CallRequest.CreatedOn,
		State:     r.CallRequest.State,
	}
}

// CallRequestStateInfo holds the state of a call request.
type CallRequestStateInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// CallRequestCase is a compact reference to the case a call request belongs to.
type CallRequestCase struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Number *string `json:"number,omitempty"`
}

// CallRequestSummary is one item of the portal's response for
// POST /call-requests/search. Assignee/Notes/Plan/Attendees/ActionItems/
// ActualDurationMin are agent-side fields entity-service populates once a
// support engineer schedules or concludes the call — read-only information
// for the customer, not something they set (see
// dto.CallRequestUpdateRequest for the write-side restriction).
type CallRequestSummary struct {
	ID                 string               `json:"id"`
	Number             string               `json:"number"`
	Case               CallRequestCase      `json:"case"`
	Reason             *string              `json:"reason,omitempty"`
	PreferredTimes     []string             `json:"preferredTimes,omitempty"`
	DurationMin        int                  `json:"durationMin"`
	ScheduleTime       *string              `json:"scheduleTime,omitempty"`
	MeetingLink        *string              `json:"meetingLink,omitempty"`
	CreatedOn          string               `json:"createdOn"`
	UpdatedOn          string               `json:"updatedOn"`
	State              CallRequestStateInfo `json:"state"`
	CancellationReason *string              `json:"cancellationReason,omitempty"`
	Assignee           *string              `json:"assignee,omitempty"`
	Notes              *string              `json:"notes,omitempty"`
	Plan               *string              `json:"plan,omitempty"`
	Attendees          *string              `json:"attendees,omitempty"`
	ActionItems        *string              `json:"actionItems,omitempty"`
	ActualDurationMin  *int                 `json:"actualDurationMin,omitempty"`
}

// SearchCallRequestsResponse is the portal's response for
// POST /cases/{caseId}/call-requests/search. TotalRecords (not Total) to
// match the frontend's shared pagination envelope — see
// PaginationResponse in apps/customer-portal/webapp/src/types/common.ts.
type SearchCallRequestsResponse struct {
	CallRequests []CallRequestSummary `json:"callRequests"`
	TotalRecords int                  `json:"totalRecords"`
	Offset       int                  `json:"offset"`
	Limit        int                  `json:"limit"`
}

// MapSearchCallRequests builds the portal response from entity-service's SearchCallRequestsResponse.
func MapSearchCallRequests(r entity.SearchCallRequestsResponse) SearchCallRequestsResponse {
	items := make([]CallRequestSummary, 0, len(r.CallRequests))
	for _, v := range r.CallRequests {
		items = append(items, CallRequestSummary{
			ID:                 v.ID,
			Number:             v.Number,
			Case:               CallRequestCase{ID: v.Case.ID, Name: v.Case.Name, Number: v.Case.Number},
			Reason:             v.Reason,
			PreferredTimes:     v.PreferredTimes,
			DurationMin:        v.DurationMin,
			ScheduleTime:       v.ScheduleTime,
			MeetingLink:        v.MeetingLink,
			CreatedOn:          v.CreatedOn,
			UpdatedOn:          v.UpdatedOn,
			State:              CallRequestStateInfo{ID: v.State.ID, Label: v.State.Label},
			CancellationReason: v.CancellationReason,
			Assignee:           v.Assignee,
			Notes:              v.Notes,
			Plan:               v.Plan,
			Attendees:          v.Attendees,
			ActionItems:        v.ActionItems,
			ActualDurationMin:  v.ActualDurationMin,
		})
	}
	return SearchCallRequestsResponse{
		CallRequests: items,
		TotalRecords: r.Total,
		Offset:       r.Offset,
		Limit:        r.Limit,
	}
}

// CallRequestUpdateRequest is the portal's request shape for
// PATCH /call-requests/{id} — a deliberately restricted subset of
// entity-service's UpdateCallRequestRequest. Excluded fields are agent-side
// actions entity-service itself documents as "set when an engineer schedules
// or concludes the call" — MeetingDate, Assignee, Notes, Plan, Attendees,
// ActionItems, ActualDurationMin — none of which a customer should be able
// to set on their own call request.
type CallRequestUpdateRequest struct {
	State              string   `json:"state"`
	CancellationReason *string  `json:"cancellationReason,omitempty"`
	UTCTimes           []string `json:"utcTimes,omitempty"`
	DurationMinutes    *int     `json:"durationInMinutes,omitempty"`
}

// BuildEntityUpdateCallRequestRequest converts the portal's restricted update
// request into entity-service's full request shape, leaving every excluded
// (agent-side) field nil.
func BuildEntityUpdateCallRequestRequest(req CallRequestUpdateRequest) entity.UpdateCallRequestRequest {
	return entity.UpdateCallRequestRequest{
		State:              req.State,
		CancellationReason: req.CancellationReason,
		UTCTimes:           req.UTCTimes,
		DurationMinutes:    req.DurationMinutes,
	}
}

// CallRequestUpdateResponse is the portal's response for PATCH /call-requests/{id}.
// Deliberately excludes entity-service's UpdatedBy (internal actor identity),
// consistent with the other update responses in this package.
type CallRequestUpdateResponse struct {
	ID        string `json:"id"`
	UpdatedOn string `json:"updatedOn"`
}

// MapCallRequestUpdate builds the portal response from entity-service's UpdateCallRequestResponse.
func MapCallRequestUpdate(r entity.UpdateCallRequestResponse) CallRequestUpdateResponse {
	return CallRequestUpdateResponse{
		ID:        r.CallRequest.ID,
		UpdatedOn: r.CallRequest.UpdatedOn,
	}
}
