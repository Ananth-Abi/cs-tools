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

import (
	"context"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// validScheduledTaskRunStatusFilter is the allow-list for List's
// statusFilter — "" means no filter (every row).
var validScheduledTaskRunStatusFilter = map[string]bool{"": true, "failed": true, "succeeded": true, "superseded": true}

type scheduledTaskRunService struct {
	repo repository.ScheduledTaskRunRepository
}

// NewScheduledTaskRunService constructs a ScheduledTaskRunService backed by
// the given repository.
func NewScheduledTaskRunService(repo repository.ScheduledTaskRunRepository) ScheduledTaskRunService {
	return &scheduledTaskRunService{repo: repo}
}

// Attempt implements ScheduledTaskRunService.
func (s *scheduledTaskRunService) Attempt(ctx context.Context, req domain.ClaimScheduledTaskRunRequest) (domain.ClaimScheduledTaskRunResponse, error) {
	if req.TaskName == "" {
		return domain.ClaimScheduledTaskRunResponse{}, &apierror.ValidationError{Msg: "taskName is required"}
	}
	if req.PeriodKey.IsZero() {
		return domain.ClaimScheduledTaskRunResponse{}, &apierror.ValidationError{Msg: "periodKey is required"}
	}
	run, allowed, err := s.repo.Attempt(ctx, req)
	if err != nil {
		return domain.ClaimScheduledTaskRunResponse{}, err
	}
	return domain.ClaimScheduledTaskRunResponse{Allowed: allowed, Run: run}, nil
}

// Complete implements ScheduledTaskRunService.
func (s *scheduledTaskRunService) Complete(ctx context.Context, id string) (domain.ScheduledTaskRun, error) {
	if id == "" {
		return domain.ScheduledTaskRun{}, &apierror.ValidationError{Msg: "id is required"}
	}
	return s.repo.Complete(ctx, id)
}

// Fail implements ScheduledTaskRunService.
func (s *scheduledTaskRunService) Fail(ctx context.Context, id string, req domain.FailScheduledTaskRunRequest) (domain.ScheduledTaskRun, error) {
	if id == "" {
		return domain.ScheduledTaskRun{}, &apierror.ValidationError{Msg: "id is required"}
	}
	if req.Error == "" {
		return domain.ScheduledTaskRun{}, &apierror.ValidationError{Msg: "error is required"}
	}
	if req.NextRetryOn.IsZero() {
		return domain.ScheduledTaskRun{}, &apierror.ValidationError{Msg: "nextRetryOn is required"}
	}
	return s.repo.Fail(ctx, id, req)
}

// List implements ScheduledTaskRunService.
func (s *scheduledTaskRunService) List(ctx context.Context, statusFilter string) (domain.ListScheduledTaskRunsResponse, error) {
	if !validScheduledTaskRunStatusFilter[statusFilter] {
		return domain.ListScheduledTaskRunsResponse{}, &apierror.ValidationError{Msg: "status must be one of failed, succeeded, superseded"}
	}
	runs, err := s.repo.List(ctx, statusFilter)
	if err != nil {
		return domain.ListScheduledTaskRunsResponse{}, err
	}
	return domain.ListScheduledTaskRunsResponse{Runs: runs}, nil
}

// DeleteResolvedBefore implements ScheduledTaskRunService.
func (s *scheduledTaskRunService) DeleteResolvedBefore(ctx context.Context, cutoff time.Time) (domain.DeleteScheduledTaskRunsResponse, error) {
	if cutoff.IsZero() {
		return domain.DeleteScheduledTaskRunsResponse{}, &apierror.ValidationError{Msg: "resolvedBefore is required"}
	}
	n, err := s.repo.DeleteResolvedBefore(ctx, cutoff)
	if err != nil {
		return domain.DeleteScheduledTaskRunsResponse{}, err
	}
	return domain.DeleteScheduledTaskRunsResponse{DeletedCount: n}, nil
}
