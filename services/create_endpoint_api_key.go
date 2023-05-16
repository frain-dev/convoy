package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/xdg-go/pbkdf2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth"
	"github.com/oklog/ulid/v2"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

type CreateEndpointAPIKeyService struct {
	APIKeyRepo datastore.APIKeyRepository
	D          *models.CreateEndpointApiKey
}

func (ss *CreateEndpointAPIKeyService) Run(ctx context.Context) (*datastore.APIKey, string, error) {
	if ss.D.Endpoint.ProjectID != ss.D.Project.UID {
		return nil, "", &ServiceError{ErrMsg: "endpoint does not belong to project"}
	}

	role := auth.Role{
		Type:     auth.RoleAdmin,
		Project:  ss.D.Project.UID,
		Endpoint: ss.D.Endpoint.UID,
	}

	maskID, key := util.GenerateAPIKey()
	salt, err := util.GenerateSecret()
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate salt")
		return nil, "", &ServiceError{ErrMsg: "something went wrong"}
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	var v time.Time
	if ss.D.KeyType == datastore.CLIKey {
		v = time.Now().Add(time.Hour * 24 * time.Duration(ss.D.Expiration))
	} else if ss.D.KeyType == datastore.AppPortalKey {
		v = time.Now().Add(30 * time.Minute)
	}

	apiKey := &datastore.APIKey{
		UID:       ulid.Make().String(),
		MaskID:    maskID,
		Name:      ss.D.Name,
		Type:      ss.D.KeyType,
		Role:      role,
		Hash:      encodedKey,
		Salt:      salt,
		ExpiresAt: null.NewTime(v, true),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = ss.APIKeyRepo.CreateAPIKey(ctx, apiKey)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create api key")
		return nil, "", &ServiceError{ErrMsg: "failed to create api key", Err: err}
	}

	return apiKey, key, nil
}
