package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/license"

	"github.com/frain-dev/convoy/auth"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

type ProjectService struct {
	apiKeyRepo        datastore.APIKeyRepository
	projectRepo       datastore.ProjectRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	eventTypesRepo    datastore.EventTypesRepository
	Licenser          license.Licenser
	cache             cache.Cache
}

func NewProjectService(apiKeyRepo datastore.APIKeyRepository, projectRepo datastore.ProjectRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, licenser license.Licenser, cache cache.Cache, eventTypesRepo datastore.EventTypesRepository) (*ProjectService, error) {
	return &ProjectService{
		apiKeyRepo:        apiKeyRepo,
		projectRepo:       projectRepo,
		eventRepo:         eventRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		eventTypesRepo:    eventTypesRepo,
		Licenser:          licenser,
		cache:             cache,
	}, nil
}

var ErrProjectLimit = errors.New("your instance has reached it's project limit, upgrade to create more projects")

func (ps *ProjectService) CreateProject(ctx context.Context, newProject *models.CreateProject, org *datastore.Organisation, member *datastore.OrganisationMember) (*datastore.Project, *models.APIKeyResponse, error) {
	ok, err := ps.Licenser.CreateProject(ctx)
	if err != nil {
		return nil, nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if !ok {
		return nil, nil, util.NewServiceError(http.StatusBadRequest, ErrProjectLimit)
	}

	projectName := newProject.Name

	projectConfig := newProject.Config.Transform()
	if projectConfig == nil {
		projectConfig = &datastore.DefaultProjectConfig
	} else {
		if projectConfig.Signature != nil {
			checkSignatureVersions(projectConfig.Signature.Versions)
		} else {
			projectConfig.Signature = datastore.DefaultProjectConfig.Signature
		}

		if projectConfig.RateLimit == nil {
			projectConfig.RateLimit = datastore.DefaultProjectConfig.RateLimit
		}

		if projectConfig.Strategy == nil {
			projectConfig.Strategy = datastore.DefaultProjectConfig.Strategy
		}

		if projectConfig.SSL == nil {
			projectConfig.SSL = &datastore.DefaultSSLConfig
		}

		err := validateMetaEvent(projectConfig)
		if err != nil {
			return nil, nil, util.NewServiceError(http.StatusBadRequest, err)
		}

		if !util.IsStringEmpty(projectConfig.SearchPolicy) {
			_, err = time.ParseDuration(projectConfig.SearchPolicy)
			if err != nil {
				return nil, nil, util.NewServiceError(http.StatusBadRequest, err)
			}
		}
	}

	if !ps.Licenser.AdvancedWebhookFiltering() {
		projectConfig.SearchPolicy = ""
	}

	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           projectName,
		Type:           datastore.ProjectType(newProject.Type),
		OrganisationID: org.UID,
		Config:         projectConfig,
		LogoURL:        newProject.LogoURL,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err = ps.projectRepo.CreateProject(ctx, project)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create project")
		if errors.Is(err, datastore.ErrDuplicateProjectName) {
			return nil, nil, util.NewServiceError(http.StatusBadRequest, err)
		}

		return nil, nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create project"))
	}

	err = ps.eventTypesRepo.CreateDefaultEventType(ctx, project.UID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create default event types")
	}

	newAPIKey := &models.APIKey{
		Name: fmt.Sprintf("%s's default key", project.Name),
		Role: models.Role{
			Type:    auth.RoleAdmin,
			Project: project.UID,
		},
	}

	cak := CreateAPIKeyService{
		ProjectRepo: ps.projectRepo,
		APIKeyRepo:  ps.apiKeyRepo,
		Member:      member,
		NewApiKey:   newAPIKey,
	}

	apiKey, keyString, err := cak.Run(ctx)
	if err != nil {
		return nil, nil, err
	}

	resp := &models.APIKeyResponse{
		APIKey: models.APIKey{
			Name: apiKey.Name,
			Role: models.Role{
				Type:    apiKey.Role.Type,
				Project: apiKey.Role.Project,
			},
			Type:      apiKey.Type,
			ExpiresAt: apiKey.ExpiresAt,
		},
		UID:       apiKey.UID,
		CreatedAt: apiKey.CreatedAt,
		Key:       keyString,
	}

	// if this is a community license, add this project to list of enabled projects
	// because if the initial license check above passed, then the project count limit had
	// not been reached
	ps.Licenser.AddEnabledProject(project.UID)

	return project, resp, nil
}

func (ps *ProjectService) UpdateProject(ctx context.Context, project *datastore.Project, update *models.UpdateProject) (*datastore.Project, error) {
	if !util.IsStringEmpty(update.Name) {
		project.Name = update.Name
	}

	if update.Config != nil {
		if !util.IsStringEmpty(update.Config.SearchPolicy) {
			_, err := time.ParseDuration(update.Config.SearchPolicy)
			if err != nil {
				return nil, util.NewServiceError(http.StatusBadRequest, err)
			}
		}

		project.Config = update.Config.Transform()
		checkSignatureVersions(project.Config.Signature.Versions)
		err := validateMetaEvent(project.Config)
		if err != nil {
			return nil, util.NewServiceError(http.StatusBadRequest, err)
		}
	}

	if !util.IsStringEmpty(update.LogoURL) {
		project.LogoURL = update.LogoURL
	}

	if !ps.Licenser.AdvancedWebhookFiltering() {
		project.Config.SearchPolicy = ""
	}

	err := ps.projectRepo.UpdateProject(ctx, project)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to to update project")
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	return project, nil
}

func checkSignatureVersions(versions []datastore.SignatureVersion) {
	for i := range versions {
		v := &versions[i]
		if v.UID == "" {
			v.UID = ulid.Make().String()
		}

		if v.CreatedAt.Unix() == 0 {
			v.CreatedAt = time.Now()
		}
	}
}

func validateMetaEvent(c *datastore.ProjectConfig) error {
	metaEvent := c.MetaEvent
	if metaEvent == nil {
		return nil
	}

	if !metaEvent.IsEnabled {
		return nil
	}

	if metaEvent.Type == datastore.HTTPMetaEvent {
		url, err := util.ValidateEndpoint(metaEvent.URL, c.SSL.EnforceSecureEndpoints)
		if err != nil {
			return err
		}
		metaEvent.URL = url
	}

	if util.IsStringEmpty(metaEvent.Secret) {
		sc, err := util.GenerateSecret()
		if err != nil {
			return err
		}

		metaEvent.Secret = sc
	}

	return nil
}
