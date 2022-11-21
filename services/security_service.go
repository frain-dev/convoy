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
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/xdg-go/pbkdf2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SecurityService struct {
	groupRepo  datastore.GroupRepository
	apiKeyRepo datastore.APIKeyRepository
}

func NewSecurityService(groupRepo datastore.GroupRepository, apiKeyRepo datastore.APIKeyRepository) *SecurityService {
	return &SecurityService{groupRepo: groupRepo, apiKeyRepo: apiKeyRepo}
}

func (ss *SecurityService) CreateAPIKey(ctx context.Context, member *datastore.OrganisationMember, newApiKey *models.APIKey) (*datastore.APIKey, string, error) {
	if newApiKey.ExpiresAt != (time.Time{}) && newApiKey.ExpiresAt.Before(time.Now()) {
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("expiry date is invalid"))
	}

	role := &auth.Role{
		Type:  newApiKey.Role.Type,
		Group: newApiKey.Role.Group,
	}

	err := role.Validate("api key")
	if err != nil {
		log.WithError(err).Error("invalid api key role")
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("invalid api key role"))
	}

	group, err := ss.groupRepo.FetchGroupByID(ctx, newApiKey.Role.Group)
	if err != nil {
		log.WithError(err).Error("failed to fetch group by id")
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch group by id"))
	}

	// does the group belong to the member's organisation?
	if group.OrganisationID != member.OrganisationID {
		return nil, "", util.NewServiceError(http.StatusUnauthorized, errors.New("unauthorized to access group"))
	}

	// does the organisation member have access to this group they're trying to create an api key for?
	if !member.Role.Type.Is(auth.RoleSuperUser) && !member.Role.HasGroup(group.UID) {
		return nil, "", util.NewServiceError(http.StatusUnauthorized, errors.New("unauthorized to access group"))
	}

	maskID, key := util.GenerateAPIKey()

	salt, err := util.GenerateSecret()
	if err != nil {
		log.WithError(err).Error("failed to generate salt")
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("something went wrong"))
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	apiKey := &datastore.APIKey{
		UID:            uuid.New().String(),
		MaskID:         maskID,
		Name:           newApiKey.Name,
		Type:           newApiKey.Type, // TODO: this should be set to datastore.ProjectKey
		Role:           *role,
		Hash:           encodedKey,
		Salt:           salt,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	if newApiKey.ExpiresAt != (time.Time{}) {
		apiKey.ExpiresAt = primitive.NewDateTimeFromTime(newApiKey.ExpiresAt)
	}

	err = ss.apiKeyRepo.CreateAPIKey(ctx, apiKey)
	if err != nil {
		log.WithError(err).Error("failed to create api key")
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("failed to create api key"))
	}

	return apiKey, key, nil
}

func (ss *SecurityService) CreatePersonalAPIKey(ctx context.Context, user *datastore.User, newApiKey *models.PersonalAPIKey) (*datastore.APIKey, string, error) {
	maskID, key := util.GenerateAPIKey()

	salt, err := util.GenerateSecret()
	if err != nil {
		log.WithError(err).Error("failed to generate salt")
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
		UID:            uuid.New().String(),
		MaskID:         maskID,
		Name:           newApiKey.Name,
		Type:           datastore.PersonalKey,
		UserID:         user.UID,
		Hash:           encodedKey,
		Salt:           salt,
		ExpiresAt:      primitive.NewDateTimeFromTime(expiresAt),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	err = ss.apiKeyRepo.CreateAPIKey(ctx, apiKey)
	if err != nil {
		log.WithError(err).Error("failed to create api key")
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
		log.WithError(err).Error("failed to fetch api key")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch api key"))
	}

	if apiKey.Type != datastore.PersonalKey || apiKey.UserID != user.UID {
		return util.NewServiceError(http.StatusUnauthorized, errors.New("unauthorized"))
	}

	err = ss.apiKeyRepo.RevokeAPIKeys(ctx, []string{uid})
	if err != nil {
		log.WithError(err).Error("failed to revoke api key")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to revoke api key"))
	}

	return nil
}

func (ss *SecurityService) CreateAppAPIKey(ctx context.Context, d *models.CreateAppApiKey) (*datastore.APIKey, string, error) {
	if d.App.GroupID != d.Group.UID {
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("app does not belong to group"))
	}

	role := auth.Role{
		Type:  auth.RoleAdmin,
		Group: d.Group.UID,
		App:   d.App.UID,
	}

	maskID, key := util.GenerateAPIKey()
	salt, err := util.GenerateSecret()
	if err != nil {
		log.WithError(err).Error("failed to generate salt")
		return nil, "", util.NewServiceError(http.StatusBadRequest, errors.New("something went wrong"))
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	var expiresAt time.Time
	if d.KeyType == datastore.CLIKey {
		expiresAt = time.Now().Add(time.Hour * 24 * time.Duration(d.Expiration))
	} else if d.KeyType == datastore.AppPortalKey {
		expiresAt = time.Now().Add(time.Hour * 24 * 30)
	}

	apiKey := &datastore.APIKey{
		UID:            uuid.New().String(),
		MaskID:         maskID,
		Name:           d.Name,
		Type:           d.KeyType,
		Role:           role,
		Hash:           encodedKey,
		Salt:           salt,
		DocumentStatus: datastore.ActiveDocumentStatus,
		ExpiresAt:      primitive.NewDateTimeFromTime(expiresAt),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	err = ss.apiKeyRepo.CreateAPIKey(ctx, apiKey)
	if err != nil {
		log.WithError(err).Error("failed to create api key")
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
		log.WithError(err).Error("failed to revoke api key")
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
		log.WithError(err).Error("failed to fetch api key")
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
		log.WithError(err).Error("invalid api key role")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("invalid api key role"))
	}

	_, err = ss.groupRepo.FetchGroupByID(ctx, role.Group)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("invalid group"))
	}

	apiKey, err := ss.apiKeyRepo.FindAPIKeyByID(ctx, uid)
	if err != nil {
		log.WithError(err).Error("failed to fetch api key")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch api key"))
	}

	apiKey.Role = *role
	err = ss.apiKeyRepo.UpdateAPIKey(ctx, apiKey)
	if err != nil {
		log.WithError(err).Error("failed to update api key")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update api key"))
	}

	return apiKey, nil
}

func (ss *SecurityService) GetAPIKeys(ctx context.Context, f *datastore.ApiKeyFilter, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	apiKeys, paginationData, err := ss.apiKeyRepo.LoadAPIKeysPaged(ctx, f, pageable)
	if err != nil {
		log.WithError(err).Error("failed to load api keys")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusBadRequest, errors.New("failed to load api keys"))
	}

	return apiKeys, paginationData, nil
}
