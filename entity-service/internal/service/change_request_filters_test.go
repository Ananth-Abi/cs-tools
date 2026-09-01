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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestParseChangeRequestFieldFilters_CreatedOnRelativeDate(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

	parsed, err := ParseChangeRequestFieldFilters([]domain.ChangeRequestFieldFilter{
		{Field: "createdOn", Op: "gte", Values: []string{"__daysAgo:90__"}},
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.CreatedStartDate == nil {
		t.Fatal("CreatedStartDate = nil, want a resolved time")
	}
	want := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	if !parsed.CreatedStartDate.Equal(want) {
		t.Errorf("CreatedStartDate = %v, want %v", parsed.CreatedStartDate, want)
	}
}

func TestParseChangeRequestFieldFilters_CreatedOnAbsoluteDateStillWorks(t *testing.T) {
	now := time.Now().UTC()

	parsed, err := ParseChangeRequestFieldFilters([]domain.ChangeRequestFieldFilter{
		{Field: "createdOn", Op: "lte", Values: []string{"2026-01-01"}},
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.CreatedEndDate == nil {
		t.Fatal("CreatedEndDate = nil, want a resolved time")
	}
}
