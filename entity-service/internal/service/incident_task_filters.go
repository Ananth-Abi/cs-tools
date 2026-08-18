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
	"strconv"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// incidentTaskFilterFieldSet is the exact set of IncidentTaskFieldFilter.Field
// values accepted by incident-task search. Anything else is rejected outright.
var incidentTaskFilterFieldSet = map[string]bool{
	"state": true, "assignmentGroupId": true, "incidentId": true,
}

// incidentTaskFilterOpSet is the exact set of IncidentTaskFieldFilter.Op
// values accepted by incident-task search, independent of field.
var incidentTaskFilterOpSet = map[string]bool{
	"in": true,
}

// requireIncidentTaskFilterValues rejects a filter entry whose op needs a
// non-empty values array but doesn't have one.
func requireIncidentTaskFilterValues(f domain.IncidentTaskFieldFilter) error {
	if len(f.Values) == 0 {
		return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q op %q requires a non-empty values array", f.Field, f.Op)}
	}
	return nil
}

// badIncidentTaskFilterCombo reports a field/op combination that is not supported.
func badIncidentTaskFilterCombo(f domain.IncidentTaskFieldFilter) error {
	return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q does not support op %q", f.Field, f.Op)}
}

// parsedIncidentTaskFilters is the internal, named-field representation that
// SearchIncidentTasksFilters.Filters is translated into by
// ParseIncidentTaskFieldFilters.
type parsedIncidentTaskFilters struct {
	// StateKeys are raw integer state values, passed through as-is -- NOT
	// translated from a domain enum. incident_task has no confirmed-complete,
	// unambiguous state enum to translate through (its state choice list is
	// shared with the data source's base task table and is inconsistent
	// across task subtypes, with overlapping/ambiguous values), so this field
	// deliberately skips the enum-translation layer every other resource's
	// state filter goes through. See domain.SearchIncidentTasksFilters.Filters.
	StateKeys []int
	// AssignmentGroupIDs are sys_user_group UUIDs (not yet converted to
	// sysids -- that conversion happens where the outbound payload is built).
	AssignmentGroupIDs []string
	// IncidentIDs are parent-incident UUIDs (not yet converted to sysids).
	IncidentIDs []string
}

// ParseIncidentTaskFieldFilters translates the incident-task-search wire
// contract's generic filter array (domain.IncidentTaskFieldFilter) into
// parsedIncidentTaskFilters, mirroring ParseProblemFieldFilters in
// problem_filters.go.
func ParseIncidentTaskFieldFilters(filters []domain.IncidentTaskFieldFilter) (parsedIncidentTaskFilters, error) {
	var p parsedIncidentTaskFilters

	for _, f := range filters {
		if !incidentTaskFilterFieldSet[f.Field] {
			return parsedIncidentTaskFilters{}, &apierror.ValidationError{Msg: "filters: unsupported field: " + f.Field}
		}
		if !incidentTaskFilterOpSet[f.Op] {
			return parsedIncidentTaskFilters{}, &apierror.ValidationError{Msg: "filters: unsupported op: " + f.Op}
		}

		switch f.Field {
		case "state":
			if f.Op != "in" {
				return parsedIncidentTaskFilters{}, badIncidentTaskFilterCombo(f)
			}
			if err := requireIncidentTaskFilterValues(f); err != nil {
				return parsedIncidentTaskFilters{}, err
			}
			for _, v := range f.Values {
				n, err := strconv.Atoi(v)
				if err != nil {
					return parsedIncidentTaskFilters{}, &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q value %q must be an integer", f.Field, v)}
				}
				p.StateKeys = append(p.StateKeys, n)
			}

		case "assignmentGroupId":
			if f.Op != "in" {
				return parsedIncidentTaskFilters{}, badIncidentTaskFilterCombo(f)
			}
			if err := requireIncidentTaskFilterValues(f); err != nil {
				return parsedIncidentTaskFilters{}, err
			}
			if err := validateUUIDs("filters: assignmentGroupId", f.Values); err != nil {
				return parsedIncidentTaskFilters{}, err
			}
			p.AssignmentGroupIDs = append(p.AssignmentGroupIDs, f.Values...)

		case "incidentId":
			if f.Op != "in" {
				return parsedIncidentTaskFilters{}, badIncidentTaskFilterCombo(f)
			}
			if err := requireIncidentTaskFilterValues(f); err != nil {
				return parsedIncidentTaskFilters{}, err
			}
			if err := validateUUIDs("filters: incidentId", f.Values); err != nil {
				return parsedIncidentTaskFilters{}, err
			}
			p.IncidentIDs = append(p.IncidentIDs, f.Values...)
		}
	}

	return p, nil
}
