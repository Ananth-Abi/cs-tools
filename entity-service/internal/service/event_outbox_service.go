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

// Package service is declared in interfaces.go.
package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

var validEventOutboxStatus = map[domain.EventOutboxStatus]bool{
	domain.EventOutboxStatusWaiting:     true,
	domain.EventOutboxStatusDispatching: true,
	domain.EventOutboxStatusDispatched:  true,
}

type eventOutboxService struct {
	repo repository.EventOutboxRepository
}

// NewEventOutboxService constructs an EventOutboxService backed by the given repository.
func NewEventOutboxService(repo repository.EventOutboxRepository) EventOutboxService {
	return &eventOutboxService{repo: repo}
}

// CreateEventOutbox implements EventOutboxService.
func (s *eventOutboxService) CreateEventOutbox(ctx context.Context, req domain.CreateEventOutboxRequest) (domain.EventOutbox, error) {
	if req.EventType == "" {
		return domain.EventOutbox{}, &apierror.ValidationError{Msg: "eventType is required"}
	}
	if req.EntityID == "" {
		return domain.EventOutbox{}, &apierror.ValidationError{Msg: "entityId is required"}
	}
	if len(req.Payload) == 0 {
		return domain.EventOutbox{}, &apierror.ValidationError{Msg: "payload is required"}
	}
	return s.repo.Create(ctx, req)
}

// UpdateEventOutboxStatus implements EventOutboxService.
func (s *eventOutboxService) UpdateEventOutboxStatus(ctx context.Context, id string, req domain.UpdateEventOutboxStatusRequest) (domain.EventOutbox, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.EventOutbox{}, err
	}
	switch req.Status {
	case domain.EventOutboxStatusDispatching:
		return s.repo.Claim(ctx, id)
	case domain.EventOutboxStatusDispatched:
		return s.repo.MarkDispatched(ctx, id)
	case domain.EventOutboxStatusWaiting:
		return s.repo.ReleaseFailed(ctx, id)
	default:
		return domain.EventOutbox{}, &apierror.ValidationError{Msg: "status must be one of: dispatching, dispatched, waiting"}
	}
}

// SearchEventOutbox implements EventOutboxService.
func (s *eventOutboxService) SearchEventOutbox(ctx context.Context, req domain.SearchEventOutboxRequest) (domain.SearchEventOutboxResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchEventOutboxResponse{}, err
	}
	if req.Filters.Status != nil && !validEventOutboxStatus[*req.Filters.Status] {
		return domain.SearchEventOutboxResponse{}, &apierror.ValidationError{Msg: "filters.status contains invalid value: " + string(*req.Filters.Status)}
	}
	events, total, err := s.repo.SearchWaiting(ctx, req)
	if err != nil {
		return domain.SearchEventOutboxResponse{}, err
	}
	return domain.SearchEventOutboxResponse{
		Events:  events,
		Total:   total,
		Limit:   req.Pagination.Limit,
		Offset:  req.Pagination.Offset,
		HasMore: req.Pagination.Offset+len(events) < total,
	}, nil
}
