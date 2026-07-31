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

import (
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// AttachmentCreateResponse is the portal's response for POST /attachments.
type AttachmentCreateResponse struct {
	ID          string    `json:"id"`
	SizeBytes   int       `json:"sizeBytes"`
	CreatedOn   time.Time `json:"createdOn"`
	DownloadURL string    `json:"downloadUrl"`
}

// MapAttachmentCreate builds the portal response from entity-service's CreateAttachmentResponse.
func MapAttachmentCreate(r entity.CreateAttachmentResponse) AttachmentCreateResponse {
	return AttachmentCreateResponse{
		ID:          r.Attachment.ID,
		SizeBytes:   r.Attachment.SizeBytes,
		CreatedOn:   r.Attachment.CreatedOn,
		DownloadURL: r.Attachment.DownloadURL,
	}
}

// AttachmentSummary is one item of the portal's response for POST /attachments/search.
type AttachmentSummary struct {
	ID            string    `json:"id"`
	ReferenceID   string    `json:"referenceId"`
	ReferenceType string    `json:"referenceType"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	SizeBytes     int       `json:"sizeBytes"`
	Description   *string   `json:"description,omitempty"`
	CreatedBy     string    `json:"createdBy"`
	CreatedOn     time.Time `json:"createdOn"`
	DownloadURL   *string   `json:"downloadUrl,omitempty"`
	PreviewURL    *string   `json:"previewUrl,omitempty"`
}

// SearchAttachmentsResponse is the portal's response for POST /attachments/search.
type SearchAttachmentsResponse struct {
	Attachments []AttachmentSummary `json:"attachments"`
	Total       int                 `json:"total"`
	Limit       int                 `json:"limit"`
	Offset      int                 `json:"offset"`
	HasMore     bool                `json:"hasMore"`
}

// MapSearchAttachments builds the portal response from entity-service's SearchAttachmentsResponse.
func MapSearchAttachments(r entity.SearchAttachmentsResponse) SearchAttachmentsResponse {
	items := make([]AttachmentSummary, 0, len(r.Attachments))
	for _, a := range r.Attachments {
		items = append(items, AttachmentSummary{
			ID:            a.ID,
			ReferenceID:   a.ReferenceID,
			ReferenceType: string(a.ReferenceType),
			Name:          a.Name,
			Type:          a.Type,
			SizeBytes:     a.SizeBytes,
			Description:   a.Description,
			CreatedBy:     a.CreatedBy,
			CreatedOn:     a.CreatedOn,
			DownloadURL:   a.DownloadURL,
			PreviewURL:    a.PreviewURL,
		})
	}
	return SearchAttachmentsResponse{
		Attachments: items,
		Total:       r.Total,
		Limit:       r.Limit,
		Offset:      r.Offset,
		HasMore:     r.HasMore,
	}
}

// DeleteResponse is the portal's response for DELETE /attachments/{id}.
type DeleteResponse struct {
	Message string `json:"message"`
}

// MapDeleteAttachment builds the portal response from entity-service's DeleteAttachmentResponse.
func MapDeleteAttachment(r entity.DeleteAttachmentResponse) DeleteResponse {
	return DeleteResponse{Message: r.Message}
}
