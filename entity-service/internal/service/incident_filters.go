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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// incidentFilterFieldSet is the exact set of IncidentFieldFilter.Field values
// accepted by incident search. Anything else is rejected outright.
var incidentFilterFieldSet = map[string]bool{
	"state": true, "assignmentGroupId": true, "businessServiceId": true,
}

// incidentFilterOpSet is the exact set of IncidentFieldFilter.Op values
// accepted by incident search, independent of field. Field/op compatibility
// is enforced separately in ParseIncidentFieldFilters -- today every
// supported field only accepts "in", but the set is kept apart from the
// per-field check to mirror case_filters.go's shape.
var incidentFilterOpSet = map[string]bool{
	"in": true,
}

// requireIncidentFilterValues rejects a filter entry whose op needs a
// non-empty values array but doesn't have one.
func requireIncidentFilterValues(f domain.IncidentFieldFilter) error {
	if len(f.Values) == 0 {
		return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q op %q requires a non-empty values array", f.Field, f.Op)}
	}
	return nil
}

// badIncidentFilterCombo reports a field/op combination that is not supported.
func badIncidentFilterCombo(f domain.IncidentFieldFilter) error {
	return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q does not support op %q", f.Field, f.Op)}
}

// parsedIncidentFilters is the internal, named-field representation that
// SearchIncidentsFilters.Filters is translated into by
// ParseIncidentFieldFilters. snIncidentService.SearchIncidents builds the
// outbound ServiceNow payload from this, unchanged from how it read the old
// flat StateKeys/AssignmentGroupIDs/BusinessServiceIDs request fields.
type parsedIncidentFilters struct {
	// StateKeys are ServiceNow's raw incident_state numeric keys, already
	// translated from the wire-level domain.IncidentState enum values via
	// snIncidentStateKeyMap.
	StateKeys []int
	// AssignmentGroupIDs are sys_user_group UUIDs (not yet converted to
	// sysids -- that conversion happens where the outbound payload is built,
	// same as before).
	AssignmentGroupIDs []string
	// BusinessServiceIDs are business_service UUIDs (not yet converted to
	// sysids).
	BusinessServiceIDs []string
}

// ParseIncidentFieldFilters translates the incident-search wire contract's
// generic filter array (domain.IncidentFieldFilter) into parsedIncidentFilters,
// mirroring ParseCaseFieldFilters in case_filters.go.
func ParseIncidentFieldFilters(filters []domain.IncidentFieldFilter) (parsedIncidentFilters, error) {
	var p parsedIncidentFilters

	for _, f := range filters {
		if !incidentFilterFieldSet[f.Field] {
			return parsedIncidentFilters{}, &apierror.ValidationError{Msg: "filters: unsupported field: " + f.Field}
		}
		if !incidentFilterOpSet[f.Op] {
			return parsedIncidentFilters{}, &apierror.ValidationError{Msg: "filters: unsupported op: " + f.Op}
		}

		switch f.Field {
		case "state":
			if f.Op != "in" {
				return parsedIncidentFilters{}, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return parsedIncidentFilters{}, err
			}
			for _, v := range f.Values {
				state := domain.IncidentState(v)
				if !validIncidentState[state] {
					return parsedIncidentFilters{}, &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q value %q is not a valid incident state", f.Field, v)}
				}
				p.StateKeys = append(p.StateKeys, snIncidentStateKeyMap[state])
			}

		case "assignmentGroupId":
			if f.Op != "in" {
				return parsedIncidentFilters{}, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return parsedIncidentFilters{}, err
			}
			if err := validateUUIDs("filters: assignmentGroupId", f.Values); err != nil {
				return parsedIncidentFilters{}, err
			}
			p.AssignmentGroupIDs = append(p.AssignmentGroupIDs, f.Values...)

		case "businessServiceId":
			if f.Op != "in" {
				return parsedIncidentFilters{}, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return parsedIncidentFilters{}, err
			}
			if err := validateUUIDs("filters: businessServiceId", f.Values); err != nil {
				return parsedIncidentFilters{}, err
			}
			p.BusinessServiceIDs = append(p.BusinessServiceIDs, f.Values...)
		}
	}

	return p, nil
}
