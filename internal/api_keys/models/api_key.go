package models

import (
	"errors"
	"time"

	"gopkg.in/guregu/null.v4"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

var ErrAPIKeyNotFound = errors.New("api key not found")

// CreateAPIKeyRequest represents the request to create an API key
type CreateAPIKeyRequest struct {
	Name      string            `json:"name" valid:"required"`
	Role      auth.Role         `json:"role" valid:"required"`
	Type      datastore.KeyType `json:"key_type" valid:"required"`
	UserID    string            `json:"user_id,omitempty"`
	ExpiresAt null.Time         `json:"expires_at,omitempty"`
}

// Validate validates the create API key request
func (r *CreateAPIKeyRequest) Validate() error {
	err := validation.ValidateStruct(r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Type, validation.Required,
			validation.In(datastore.PersonalKey, datastore.ProjectKey)),
	)
	if err != nil {
		return err
	}

	if !r.Type.IsValid() {
		return errors.New("invalid key type")
	}

	// Validate role type
	if util.IsStringEmpty(string(r.Role.Type)) {
		return errors.New("role type is required")
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
	if util.IsStringEmpty(r.Name) {
		return errors.New("name is required")
	}

	// Validate role type
	if util.IsStringEmpty(string(r.Role.Type)) {
		return errors.New("role type is required")
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
	if util.IsStringEmpty(r.Name) {
		return errors.New("name is required")
	}

	return nil
}

type ApiKeyFilter struct {
	ProjectID   string
	EndpointID  string
	EndpointIDs []string
	UserID      string
	KeyType     datastore.KeyType
}

type APIKey struct {
	Name      string            `json:"name"`
	Role      Role              `json:"role"`
	Type      datastore.KeyType `json:"key_type"`
	ExpiresAt null.Time         `json:"expires_at"`
}

type Role struct {
	Type    auth.RoleType `json:"type"`
	Project string        `json:"project"`
	App     string        `json:"app,omitempty"`
}

type APIKeyResponse struct {
	APIKey
	Key       string    `json:"key"`
	UID       string    `json:"uid"`
	UserID    string    `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
