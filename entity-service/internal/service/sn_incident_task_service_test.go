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

package service

import "testing"

// TestMapSNIncidentTaskDetailToView_OmitsNumberAndSubjectWhenAbsent verifies
// that an upstream payload omitting "number"/"subject" maps to nil (-> JSON
// null) on the view, not an empty string. Both fields decode to a nil
// pointer, not "", when absent from the JSON now that
// snIncidentTaskDetailResponse declares them as *string.
func TestMapSNIncidentTaskDetailToView_OmitsNumberAndSubjectWhenAbsent(t *testing.T) {
	it := snIncidentTaskDetailResponse{ID: "sys123"}

	view := mapSNIncidentTaskDetailToView(it)

	if view.Number != nil {
		t.Fatalf("expected Number to be nil when absent from the upstream payload, got %q", *view.Number)
	}
	if view.Subject != nil {
		t.Fatalf("expected Subject to be nil when absent from the upstream payload, got %q", *view.Subject)
	}
}

// TestMapSNIncidentTaskDetailToView_PassesThroughNumberAndSubjectWhenPresent
// verifies present values still map straight through.
func TestMapSNIncidentTaskDetailToView_PassesThroughNumberAndSubjectWhenPresent(t *testing.T) {
	number := "TASK0012345"
	subject := "Reboot affected node"
	it := snIncidentTaskDetailResponse{ID: "sys123", Number: &number, Subject: &subject}

	view := mapSNIncidentTaskDetailToView(it)

	if view.Number == nil || *view.Number != number {
		t.Fatalf("expected Number %q, got %v", number, view.Number)
	}
	if view.Subject == nil || *view.Subject != subject {
		t.Fatalf("expected Subject %q, got %v", subject, view.Subject)
	}
}

// TestMapSNIncidentTaskDetailToView_MapsOpenedOnClosedOn verifies the
// upstream wire fields (openedAt/closedAt, unchanged -- that's the real SN
// payload shape) map onto the response's own openedOn/closedOn fields,
// which follow this API's established naming for every other date-only
// response field.
func TestMapSNIncidentTaskDetailToView_MapsOpenedOnClosedOn(t *testing.T) {
	opened := "2026-08-01 10:00:00"
	closed := "2026-08-02 12:00:00"
	it := snIncidentTaskDetailResponse{ID: "sys123", OpenedAt: &opened, ClosedAt: &closed}

	view := mapSNIncidentTaskDetailToView(it)

	if view.OpenedOn == nil || *view.OpenedOn != opened {
		t.Fatalf("expected OpenedOn %q, got %v", opened, view.OpenedOn)
	}
	if view.ClosedOn == nil || *view.ClosedOn != closed {
		t.Fatalf("expected ClosedOn %q, got %v", closed, view.ClosedOn)
	}
}
