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

// Package dto defines the portal-facing response shapes returned to the
// customer-portal frontend, together with Map* functions that translate
// entity-service's raw response structs into them. This is the Go analogue
// of the Ballerina backend's "types" module + utils.bal mapper functions:
// the frontend never sees an entity-service struct directly, only these.
package dto

import "github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"

// UserMeResponse is the portal's response for GET /users/me.
type UserMeResponse struct {
	ID        string   `json:"id"`
	Email     string   `json:"email"`
	FirstName *string  `json:"firstName,omitempty"`
	LastName  string   `json:"lastName"`
	TimeZone  *string  `json:"timeZone,omitempty"`
	Roles     []string `json:"roles"`
}

// MapUserMe builds the portal response from entity-service's GetUserMeResponse.
func MapUserMe(u entity.GetUserMeResponse) UserMeResponse {
	return UserMeResponse{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		TimeZone:  u.TimeZone,
		Roles:     u.Roles,
	}
}
