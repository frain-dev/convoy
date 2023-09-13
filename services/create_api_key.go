package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/xdg-go/pbkdf2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
)

type CreateAPIKeyService struct {
	ProjectRepo datastore.ProjectRepository
	APIKeyRepo  datastore.APIKeyRepository

	Member    *datastore.OrganisationMember
	NewApiKey *models.APIKey
}

func (ss *CreateAPIKeyService) Run(ctx context.Context) (*datastore.APIKey, string, error) {
	if !ss.NewApiKey.ExpiresAt.IsZero() && ss.NewApiKey.ExpiresAt.ValueOrZero().Before(time.Now()) {
		return nil, "", &ServiceError{ErrMsg: "expiry date is invalid"}
	}

	role := &auth.Role{
		Type:    ss.NewApiKey.Role.Type,
		Project: ss.NewApiKey.Role.Project,
	}

	err := role.Validate("api key")
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("invalid api key role")
		return nil, "", &ServiceError{ErrMsg: "invalid api key role", Err: err}
	}

	project, err := ss.ProjectRepo.FetchProjectByID(ctx, ss.NewApiKey.Role.Project)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch project by id")
		return nil, "", &ServiceError{ErrMsg: "failed to fetch project by id", Err: err}
	}

	// does the project belong to the member's organisation?
	if project.OrganisationID != ss.Member.OrganisationID {
		return nil, "", &ServiceError{ErrMsg: "unauthorized to access project"}
	}

	// does the organisation member have access to this project they're trying to create an api key for?
	if !ss.Member.Role.Type.Is(auth.RoleSuperUser) {
		return nil, "", &ServiceError{ErrMsg: "unauthorized to access project"}
	}

	maskID, key := util.GenerateAPIKey()

	salt, err := util.GenerateSecret()
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate salt")
		return nil, "", &ServiceError{ErrMsg: "something went wrong"}
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	apiKey := &datastore.APIKey{
		UID:       ulid.Make().String(),
		MaskID:    maskID,
		Name:      ss.NewApiKey.Name,
		Type:      ss.NewApiKey.Type,
		Role:      *role,
		Hash:      encodedKey,
		Salt:      salt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if !ss.NewApiKey.ExpiresAt.IsZero() {
		apiKey.ExpiresAt = ss.NewApiKey.ExpiresAt
	}

	err = ss.APIKeyRepo.CreateAPIKey(ctx, apiKey)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create api key")
		return nil, "", &ServiceError{ErrMsg: "failed to create api key", Err: err}
	}

	return apiKey, key, nil
}
