package convoy

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/auth"
	pager "github.com/gobeam/mongo-go-pagination"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrAPIKeyNotFound = errors.New("api key not found")
)

type KeyType string

type APIKey struct {
	ID        primitive.ObjectID `json:"-" bson:"_id"`
	UID       string             `json:"uid" bson:"uid"`
	MaskID    string             `json:"-" bson:"mask_id"`
	Name      string             `json:"name" bson:"name"`
	Role      auth.Role          `json:"role" bson:"role"`
	Hash      string             `json:"-" bson:"hash"`
	Salt      string             `json:"-" bson:"salt"`
	Type      KeyType            `json:"key_type" bson:"key_type"`
	ExpiresAt primitive.DateTime `json:"expires_at,omitempty" bson:"expires_at,omitempty"`
	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at"`
	DeletedAt primitive.DateTime `json:"delted_at,omitempty" bson:"deleted_at"`

	DocumentStatus DocumentStatus `json:"-" bson:"document_status"`
}

type APIKeyRepository interface {
	CreateAPIKey(context.Context, *APIKey) error
	UpdateAPIKey(context.Context, *APIKey) error
	FindAPIKeyByID(context.Context, string) (*APIKey, error)
	FindAPIKeyByMaskID(context.Context, string) (*APIKey, error)
	FindAPIKeyByHash(context.Context, string) (*APIKey, error)
	RevokeAPIKeys(context.Context, []string) error
	LoadAPIKeysPaged(context.Context, *Pageable) ([]APIKey, *pager.PaginationData, error)
}
