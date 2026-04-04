package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/xdg-go/pbkdf2"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

type CreatePersonalAPIKeyService struct {
	ProjectRepo datastore.ProjectRepository
	UserRepo    datastore.UserRepository
	APIKeyRepo  datastore.APIKeyRepository

	User      *datastore.User
	NewApiKey *models.PersonalAPIKey
	Logger    log.Logger
}

func (cpa *CreatePersonalAPIKeyService) Run(ctx context.Context) (*datastore.APIKey, string, error) {
	maskID, key := util.GenerateAPIKey()

	salt, err := util.GenerateSecret()
	if err != nil {
		cpa.Logger.ErrorContext(ctx, "failed to generate salt", "error", err)
		return nil, "", &ServiceError{ErrMsg: "something went wrong", Err: err}
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	var v time.Time
	if cpa.NewApiKey.Expiration != 0 {
		v = time.Now().Add(time.Hour * 24 * time.Duration(cpa.NewApiKey.Expiration))
	} else {
		v = time.Now().Add(time.Hour * 24)
	}

	apiKey := &datastore.APIKey{
		UID:       ulid.Make().String(),
		MaskID:    maskID,
		Name:      cpa.NewApiKey.Name,
		Type:      datastore.PersonalKey,
		UserID:    cpa.User.UID,
		Hash:      encodedKey,
		Salt:      salt,
		ExpiresAt: null.NewTime(v, true),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = cpa.APIKeyRepo.CreateAPIKey(ctx, apiKey)
	if err != nil {
		cpa.Logger.ErrorContext(ctx, "failed to create api key", "error", err)
		return nil, "", &ServiceError{ErrMsg: "failed to create api key", Err: err}
	}

	return apiKey, key, nil
}
