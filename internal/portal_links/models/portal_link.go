package models

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"gopkg.in/guregu/null.v4"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

type QueryListPortalLink struct {
	// List of endpoint ids
	EndpointIds []string `json:"endpoint_ids"`

	// The owner ID of the endpoint
	OwnerID string `json:"ownerId" example:"01H0JA5MEES38RRK3HTEJC647K"`

	datastore.Pageable
}

type QueryListPortalLinkResponse struct {
	datastore.Pageable
	*datastore.FilterBy
}

func (q *QueryListPortalLink) Transform(r *http.Request) *QueryListPortalLinkResponse {
	return &QueryListPortalLinkResponse{
		Pageable: getPageableFromContext(r.Context()),
		FilterBy: &datastore.FilterBy{
			EndpointIDs: getEndpointIDs(r),
			OwnerID:     r.URL.Query().Get("ownerId"),
		},
	}
}

func getPageableFromContext(ctx context.Context) datastore.Pageable {
	v := ctx.Value(convoy.PageableCtx)
	if v != nil {
		return v.(datastore.Pageable)
	}
	return datastore.Pageable{}
}

func getEndpointIDs(r *http.Request) []string {
	var endpoints []string

	for _, id := range r.URL.Query()["endpointId"] {
		if !util.IsStringEmpty(id) {
			endpoints = append(endpoints, id)
		}
	}

	return endpoints
}

type PortalLinkResponse struct {
	UID               string                     `json:"uid"`
	Name              string                     `json:"name"`
	ProjectID         string                     `json:"project_id"`
	OwnerID           string                     `json:"owner_id"`
	Endpoints         []string                   `json:"endpoints"`
	EndpointCount     int                        `json:"endpoint_count"`
	CanManageEndpoint bool                       `json:"can_manage_endpoint"`
	Token             string                     `json:"token"`
	EndpointsMetadata datastore.EndpointMetadata `json:"endpoints_metadata"`
	URL               string                     `json:"url"`
	AuthType          datastore.PortalAuthType   `json:"auth_type"`
	AuthKey           string                     `json:"auth_key"`
	CreatedAt         time.Time                  `json:"created_at,omitempty"`
	UpdatedAt         time.Time                  `json:"updated_at,omitempty"`
	DeletedAt         null.Time                  `json:"deleted_at,omitempty"`
}

type UpdatePortalLinkRequest struct {
	// Portal Link Name
	Name string `json:"name" valid:"required~please provide the name field"`

	// Deprecated
	// IDs of endpoints in this portal link
	Endpoints []string `json:"endpoints"`

	AuthType string `json:"auth_type"`

	// OwnerID, the portal link will inherit all the endpoints with this owner ID
	OwnerID string `json:"owner_id"`

	// Specify whether endpoint management can be done through the Portal Link UI
	CanManageEndpoint bool `json:"can_manage_endpoint"`
}

func (p *UpdatePortalLinkRequest) Validate() error {
	validAuthTypes := []datastore.PortalAuthType{
		datastore.PortalAuthTypeRefreshToken,
		datastore.PortalAuthTypeStaticToken,
	}

	// Check if the auth type is valid
	for _, validType := range validAuthTypes {
		if validType == datastore.PortalAuthType(p.AuthType) {
			return nil
		}
	}

	return fmt.Errorf("invalid auth type: %s", p.AuthType)
}

func (p *UpdatePortalLinkRequest) SetDefaultAuthType() {
	validAuthTypes := []datastore.PortalAuthType{
		datastore.PortalAuthTypeRefreshToken,
		datastore.PortalAuthTypeStaticToken,
	}

	// Check if the auth type is valid
	for _, validType := range validAuthTypes {
		if validType == datastore.PortalAuthType(p.AuthType) {
			return
		}
	}

	// Default to refresh token
	p.AuthType = string(datastore.PortalAuthTypeStaticToken)
}

type CreatePortalLinkRequest struct {
	// Portal Link Name
	Name string `json:"name" valid:"required~please provide the name field"`

	// Deprecated
	// IDs of endpoints in this portal link
	Endpoints []string `json:"endpoints"`

	AuthType string `json:"auth_type"`

	// OwnerID, the portal link will inherit all the endpoints with this owner ID
	OwnerID string `json:"owner_id" valid:"required~please provide the owner id field"`

	// Specify whether endpoint management can be done through the Portal Link UI
	CanManageEndpoint bool `json:"can_manage_endpoint"`
}

func (p *CreatePortalLinkRequest) Validate() error {
	err := validation.ValidateStruct(p,
		validation.Field(&p.Name, validation.Required),
		validation.Field(&p.OwnerID, validation.Required),
	)

	if err != nil {
		return err
	}

	validAuthTypes := []datastore.PortalAuthType{
		datastore.PortalAuthTypeRefreshToken,
		datastore.PortalAuthTypeStaticToken,
	}

	// Check if the auth type is valid
	for _, validType := range validAuthTypes {
		if validType == datastore.PortalAuthType(p.AuthType) {
			return nil
		}
	}

	return fmt.Errorf("invalid auth type: %s", p.AuthType)
}

func (p *CreatePortalLinkRequest) SetDefaultAuthType() {
	validAuthTypes := []datastore.PortalAuthType{
		datastore.PortalAuthTypeRefreshToken,
		datastore.PortalAuthTypeStaticToken,
	}

	// Check if the auth type is valid
	for _, validType := range validAuthTypes {
		if validType == datastore.PortalAuthType(p.AuthType) {
			return
		}
	}

	// Default to static token
	p.AuthType = string(datastore.PortalAuthTypeStaticToken)
}
