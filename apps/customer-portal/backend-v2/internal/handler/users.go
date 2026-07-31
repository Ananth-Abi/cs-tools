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

package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityUserClient abstracts the entity-service user operations used by UserHandler.
type entityUserClient interface {
	GetMe(ctx context.Context) (entity.GetUserMeResponse, error)
}

// UserHandler handles HTTP requests for the logged-in user's own profile.
type UserHandler struct {
	entity entityUserClient
}

// NewUserHandler creates a UserHandler backed by the given entity client.
func NewUserHandler(entity entityUserClient) *UserHandler {
	return &UserHandler{entity: entity}
}

// GetMe handles GET /users/me.
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.entity.GetMe(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetMe failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve user profile.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapUserMe(result))
}
