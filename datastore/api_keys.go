package datastore

import (
	"errors"
	"time"

	"gopkg.in/guregu/null.v4"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/frain-dev/convoy/auth"
)

var ErrAPIKeyNotFound = errors.New("api key not found")

// CreateAPIKeyRequest represents the request to create an API key
type CreateAPIKeyRequest struct {
	Name      string    `json:"name" valid:"required"`
	Role      auth.Role `json:"role" valid:"required"`
	Type      string    `json:"key_type" valid:"required"`
	UserID    string    `json:"user_id,omitempty"`
	ExpiresAt null.Time `json:"expires_at,omitempty"`
}

// Validate validates the create API key request
func (r *CreateAPIKeyRequest) Validate() error {
	err := validation.ValidateStruct(r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Role, validation.Required),
		validation.Field(&r.Role.Type, validation.Required),
		validation.Field(&r.Type, validation.Required,
			validation.In("project", "app_portal", "cli", "personal_key")),
	)
	if err != nil {
		return err
	}

	return nil
}

// UpdateAPIKeyRequest represents the request to update an API key
type UpdateAPIKeyRequest struct {
	Name string    `json:"name" valid:"required"`
	Role auth.Role `json:"role" valid:"required"`
}

// Validate validates the update API key request
func (r *UpdateAPIKeyRequest) Validate() error {
	err := validation.ValidateStruct(r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Role, validation.Required),
		validation.Field(&r.Role.Type, validation.Required),
	)
	if err != nil {
		return err
	}

	return nil
}

// PersonalAPIKeyRequest represents the request to create a personal API key
type PersonalAPIKeyRequest struct {
	Name       string `json:"name" valid:"required"`
	Expiration int    `json:"expiration"` // in days
}

// Validate validates the personal API key request
func (r *PersonalAPIKeyRequest) Validate() error {
	err := validation.ValidateStruct(r,
		validation.Field(&r.Name, validation.Required),
	)
	if err != nil {
		return err
	}

	return nil
}

type ApiKeyFilter struct {
	ProjectID   string
	EndpointID  string
	EndpointIDs []string
	UserID      string
	KeyType     string
}

type APIKeyRes struct {
	Name      string    `json:"name"`
	Role      Role      `json:"role"`
	Type      string    `json:"key_type"`
	ExpiresAt null.Time `json:"expires_at"`
}

type Role struct {
	Type    auth.RoleType `json:"type"`
	Project string        `json:"project"`
	App     string        `json:"app,omitempty"`
}

type APIKeyResponse struct {
	APIKeyRes
	Key       string    `json:"key"`
	UID       string    `json:"uid"`
	UserID    string    `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
