package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
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
		return nil, "", NewServiceError(http.StatusBadRequest, errors.New("expiry date is invalid"))
	}

	role := &auth.Role{
		Type:   newApiKey.Role.Type,
		Groups: []string{newApiKey.Role.Group},
	}

	err := role.Validate("api key")
	if err != nil {
		log.WithError(err).Error("invalid api key role")
		return nil, "", NewServiceError(http.StatusBadRequest, errors.New("invalid api key role"))
	}

	group, err := ss.groupRepo.FetchGroupByID(ctx, newApiKey.Role.Group)
	if err != nil {
		log.WithError(err).Error("failed to fetch group by id")
		return nil, "", NewServiceError(http.StatusBadRequest, errors.New("failed to fetch group by id"))
	}

	// does the group belong to the member's organisation?
	if group.OrganisationID != member.OrganisationID {
		return nil, "", NewServiceError(http.StatusUnauthorized, errors.New("unauthorized to access group"))
	}

	// does the organisation member have access to this group they're trying to create an api key for?
	if !member.Role.Type.Is(auth.RoleSuperUser) && !member.Role.HasGroup(group.UID) {
		return nil, "", NewServiceError(http.StatusUnauthorized, errors.New("unauthorized to access group"))
	}

	maskID, key := util.GenerateAPIKey()

	salt, err := util.GenerateSecret()
	if err != nil {
		log.WithError(err).Error("failed to generate salt")
		return nil, "", NewServiceError(http.StatusBadRequest, errors.New("something went wrong"))
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	apiKey := &datastore.APIKey{
		UID:            uuid.New().String(),
		MaskID:         maskID,
		Name:           newApiKey.Name,
		Type:           newApiKey.Type,
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
		return nil, "", NewServiceError(http.StatusBadRequest, errors.New("failed to create api key"))
	}

	return apiKey, key, nil
}

func (ss *SecurityService) CreateAppPortalAPIKey(ctx context.Context, group *datastore.Group, app *datastore.Application, baseUrl *string) (*datastore.APIKey, string, error) {
	if app.GroupID != group.UID {
		return nil, "", NewServiceError(http.StatusBadRequest, errors.New("app does not belong to group"))
	}

	role := auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{group.UID},
		Apps:   []string{app.UID},
	}

	maskID, key := util.GenerateAPIKey()
	salt, err := util.GenerateSecret()

	if err != nil {
		log.WithError(err).Error("failed to generate salt")
		return nil, "", NewServiceError(http.StatusBadRequest, errors.New("something went wrong"))
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	expiresAt := time.Now().Add(30 * time.Minute)

	apiKey := &datastore.APIKey{
		UID:            uuid.New().String(),
		MaskID:         maskID,
		Name:           app.Title,
		Type:           "app_portal",
		Role:           role,
		Hash:           encodedKey,
		Salt:           salt,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
		ExpiresAt:      primitive.NewDateTimeFromTime(expiresAt),
	}

	err = ss.apiKeyRepo.CreateAPIKey(ctx, apiKey)
	if err != nil {
		log.WithError(err).Error("failed to create api key")
		return nil, "", NewServiceError(http.StatusBadRequest, errors.New("failed to create api key"))
	}

	if !util.IsStringEmpty(*baseUrl) {
		*baseUrl = fmt.Sprintf("%s/app-portal/%s?groupID=%s&appId=%s", *baseUrl, key, group.UID, app.UID)
	}

	return apiKey, key, nil
}

func (ss *SecurityService) RevokeAPIKey(ctx context.Context, uid string) error {
	if util.IsStringEmpty(uid) {
		return NewServiceError(http.StatusBadRequest, errors.New("key id is empty"))
	}

	err := ss.apiKeyRepo.RevokeAPIKeys(ctx, []string{uid})
	if err != nil {
		log.WithError(err).Error("failed to revoke api key")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to revoke api key"))
	}
	return nil
}

func (ss *SecurityService) GetAPIKeyByID(ctx context.Context, uid string) (*datastore.APIKey, error) {
	if util.IsStringEmpty(uid) {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("key id is empty"))
	}

	apiKey, err := ss.apiKeyRepo.FindAPIKeyByID(ctx, uid)
	if err != nil {
		log.WithError(err).Error("failed to fetch api key")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to fetch api key"))
	}

	return apiKey, nil
}

func (ss *SecurityService) UpdateAPIKey(ctx context.Context, uid string, role *auth.Role) (*datastore.APIKey, error) {
	if util.IsStringEmpty(uid) {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("key id is empty"))
	}

	err := role.Validate("api key")
	if err != nil {
		log.WithError(err).Error("invalid api key role")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("invalid api key role"))
	}

	groups, err := ss.groupRepo.FetchGroupsByIDs(ctx, role.Groups)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("invalid group"))
	}

	if len(groups) != len(role.Groups) {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("cannot find group"))
	}

	apiKey, err := ss.apiKeyRepo.FindAPIKeyByID(ctx, uid)
	if err != nil {
		log.WithError(err).Error("failed to fetch api key")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to fetch api key"))
	}

	apiKey.Role = *role
	err = ss.apiKeyRepo.UpdateAPIKey(ctx, apiKey)
	if err != nil {
		log.WithError(err).Error("failed to update api key")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to update api key"))
	}

	return apiKey, nil
}

func (ss *SecurityService) GetAPIKeys(ctx context.Context, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	apiKeys, paginationData, err := ss.apiKeyRepo.LoadAPIKeysPaged(ctx, pageable)
	if err != nil {
		log.WithError(err).Error("failed to load api keys")
		return nil, datastore.PaginationData{}, NewServiceError(http.StatusBadRequest, errors.New("failed to load api keys"))
	}

	return apiKeys, paginationData, nil
}
