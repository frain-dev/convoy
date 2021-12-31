package convoy

import (
	"context"

	"github.com/frain-dev/convoy/auth"
	pager "github.com/gobeam/mongo-go-pagination"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type APIKey struct {
	ID        primitive.ObjectID `json:"-" bson:"_id"`
	UID       string             `json:"uid" bson:"uid"`
	Role      auth.Role          `json:"role" bson:"role"`
	Hash      string             `json:"-" bson:"hash"`
	Revoked   bool               `json:"revoked" bson:"revoked"`
	ExpiresAt primitive.DateTime `json:"expires_at,omitempty" bson:"expires_at,omitempty"`
	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at"`
}

type APIKeyRepository interface {
	CreateAPIKey(ctx context.Context, apiKey *APIKey) error
	UpdateAPIKey(ctx context.Context, apiKey *APIKey) error
	FindAPIKeyByID(ctx context.Context, uid string) (*APIKey, error)
	FindAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error)
	RevokeAPIKeys(ctx context.Context, uids []string) error
	LoadAPIKeysPaged(ctx context.Context, pageable *Pageable) ([]APIKey, *pager.PaginationData, error)
	DeleteAPIKey(ctx context.Context, uid string) error
}
