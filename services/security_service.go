package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"

	"github.com/xdg-go/pbkdf2"
)

type SecurityService struct {
	projectRepo datastore.ProjectRepository
	apiKeyRepo  datastore.APIKeyRepository
}

func NewSecurityService(projectRepo datastore.ProjectRepository, apiKeyRepo datastore.APIKeyRepository) *SecurityService {
	return &SecurityService{projectRepo: projectRepo, apiKeyRepo: apiKeyRepo}
}

func (ss *SecurityService) CreateAPIKey(ctx context.Context, member *datastore.OrganisationMember, newApiKey *models.APIKey) (*datastore.APIKey, string, error) {
	if newApiKey.ExpiresAt != (time.Time{}) && newApiKey.ExpiresAt.Before(time.Now()) {
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("expiry date is invalid"))
	}

	role := &auth.Role{
		Type:    newApiKey.Role.Type,
		Project: newApiKey.Role.Project,
	}

	err := role.Validate("api key")
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("invalid api key role")
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("invalid api key role"))
	}

	project, err := ss.projectRepo.FetchProjectByID(ctx, newApiKey.Role.Project)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch project by id")
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch project by id"))
	}

	// does the project belong to the member's organisation?
	if project.OrganisationID != member.OrganisationID {
		return nil, "", util.NewServiceError(http.StatusUnauthorized, errors.New("unauthorized to access project"))
	}

	// does the organisation member have access to this project they're trying to create an api key for?
	if !member.Role.Type.Is(auth.RoleSuperUser) && !member.Role.HasProject(project.UID) {
		return nil, "", util.NewServiceError(http.StatusUnauthorized, errors.New("unauthorized to access project"))
	}

	maskID, key := util.GenerateAPIKey()

	salt, err := util.GenerateSecret()
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate salt")
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("something went wrong"))
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	apiKey := &datastore.APIKey{
		UID:       uuid.New().String(),
		MaskID:    maskID,
		Name:      newApiKey.Name,
		Type:      newApiKey.Type, // TODO: this should be set to datastore.ProjectKey
		Role:      *role,
		Hash:      encodedKey,
		Salt:      salt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if newApiKey.ExpiresAt != (time.Time{}) {
		apiKey.ExpiresAt = newApiKey.ExpiresAt
	}

	err = ss.apiKeyRepo.CreateAPIKey(ctx, apiKey)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create api key")
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("failed to create api key"))
	}

	return apiKey, key, nil
}

func (ss *SecurityService) CreatePersonalAPIKey(ctx context.Context, user *datastore.User, newApiKey *models.PersonalAPIKey) (*datastore.APIKey, string, error) {
	maskID, key := util.GenerateAPIKey()

	salt, err := util.GenerateSecret()
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate salt")
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("something went wrong"))
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	var expiresAt time.Time
	if newApiKey.Expiration != 0 {
		expiresAt = time.Now().Add(time.Hour * 24 * time.Duration(newApiKey.Expiration))
	} else {
		expiresAt = time.Now().Add(time.Hour * 24)
	}

	apiKey := &datastore.APIKey{
		UID:       uuid.New().String(),
		MaskID:    maskID,
		Name:      newApiKey.Name,
		Type:      datastore.PersonalKey,
		UserID:    user.UID,
		Hash:      encodedKey,
		Salt:      salt,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = ss.apiKeyRepo.CreateAPIKey(ctx, apiKey)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create api key")
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("failed to create api key"))
	}

	return apiKey, key, nil
}

func (ss *SecurityService) RevokePersonalAPIKey(ctx context.Context, uid string, user *datastore.User) error {
	if util.IsStringEmpty(uid) {
		return util.NewServiceError(http.StatusBadRequest, errors.New("key id is empty"))
	}

	apiKey, err := ss.apiKeyRepo.FindAPIKeyByID(ctx, uid)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch api key")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch api key"))
	}

	if apiKey.Type != datastore.PersonalKey || apiKey.UserID != user.UID {
		return util.NewServiceError(http.StatusUnauthorized, errors.New("unauthorized"))
	}

	err = ss.apiKeyRepo.RevokeAPIKeys(ctx, []string{uid})
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to revoke api key")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to revoke api key"))
	}

	return nil
}

func (ss *SecurityService) CreateEndpointAPIKey(ctx context.Context, d *models.CreateEndpointApiKey) (*datastore.APIKey, string, error) {
	if d.Endpoint.ProjectID != d.Project.UID {
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("endpoint does not belong to project"))
	}

	role := auth.Role{
		Type:     auth.RoleAdmin,
		Project:  d.Project.UID,
		Endpoint: d.Endpoint.UID,
	}

	maskID, key := util.GenerateAPIKey()
	salt, err := util.GenerateSecret()
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate salt")
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("something went wrong"))
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	var expiresAt time.Time
	if d.KeyType == datastore.CLIKey {
		expiresAt = time.Now().Add(time.Hour * 24 * time.Duration(d.Expiration))
	} else if d.KeyType == datastore.AppPortalKey {
		expiresAt = time.Now().Add(30 * time.Minute)
	}

	apiKey := &datastore.APIKey{
		UID:       uuid.New().String(),
		MaskID:    maskID,
		Name:      d.Name,
		Type:      d.KeyType,
		Role:      role,
		Hash:      encodedKey,
		Salt:      salt,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = ss.apiKeyRepo.CreateAPIKey(ctx, apiKey)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create api key")
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("failed to create api key"))
	}

	return apiKey, key, nil
}

func (ss *SecurityService) RevokeAPIKey(ctx context.Context, uid string) error {
	if util.IsStringEmpty(uid) {
		return util.NewServiceError(http.StatusBadRequest, errors.New("key id is empty"))
	}

	err := ss.apiKeyRepo.RevokeAPIKeys(ctx, []string{uid})
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to revoke api key")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to revoke api key"))
	}
	return nil
}

func (ss *SecurityService) GetAPIKeyByID(ctx context.Context, uid string) (*datastore.APIKey, error) {
	if util.IsStringEmpty(uid) {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("key id is empty"))
	}

	apiKey, err := ss.apiKeyRepo.FindAPIKeyByID(ctx, uid)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch api key")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch api key"))
	}

	return apiKey, nil
}

func (ss *SecurityService) UpdateAPIKey(ctx context.Context, uid string, role *auth.Role) (*datastore.APIKey, error) {
	if util.IsStringEmpty(uid) {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("key id is empty"))
	}

	err := role.Validate("api key")
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("invalid api key role")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("invalid api key role"))
	}

	_, err = ss.projectRepo.FetchProjectByID(ctx, role.Project)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("invalid project"))
	}

	apiKey, err := ss.apiKeyRepo.FindAPIKeyByID(ctx, uid)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch api key")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch api key"))
	}

	apiKey.Role = *role
	err = ss.apiKeyRepo.UpdateAPIKey(ctx, apiKey)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update api key")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update api key"))
	}

	return apiKey, nil
}

func (ss *SecurityService) GetAPIKeys(ctx context.Context, f *datastore.ApiKeyFilter, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	apiKeys, paginationData, err := ss.apiKeyRepo.LoadAPIKeysPaged(ctx, f, pageable)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to load api keys")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusBadRequest, errors.New("failed to load api keys"))
	}

	return apiKeys, paginationData, nil
}
