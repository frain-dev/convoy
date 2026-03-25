package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

type ProjectService struct {
	ApiKeyRepo        datastore.APIKeyRepository
	ProjectRepo       datastore.ProjectRepository
	EventRepo         datastore.EventRepository
	EventDeliveryRepo datastore.EventDeliveryRepository
	EventTypesRepo    datastore.EventTypesRepository
	Licenser          license.Licenser
	Logger            log.Logger
}

var ErrProjectLimit = errors.New("your instance has reached it's project limit, upgrade to create more projects")

func (ps *ProjectService) CreateProject(ctx context.Context, newProject *models.CreateProject, org *datastore.Organisation, member *datastore.OrganisationMember, skipLimitCheck bool) (*datastore.Project, *datastore.APIKeyResponse, error) {
	var err error
	if !skipLimitCheck {
		var ok bool
		ok, err = ps.Licenser.CheckProjectLimit(ctx)
		if err != nil {
			return nil, nil, util.NewServiceError(http.StatusBadRequest, err)
		}
		if !ok {
			return nil, nil, util.NewServiceError(http.StatusBadRequest, ErrProjectLimit)
		}
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

		err := validateMetaEvent(projectConfig, ps.Licenser)
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

	err = ps.ProjectRepo.CreateProject(ctx, project)
	if err != nil {
		ps.Logger.ErrorContext(ctx, "failed to create project", "error", err)
		if errors.Is(err, datastore.ErrDuplicateProjectName) {
			return nil, nil, util.NewServiceError(http.StatusBadRequest, err)
		}

		return nil, nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create project"))
	}

	err = ps.EventTypesRepo.CreateDefaultEventType(ctx, project.UID)
	if err != nil {
		ps.Logger.ErrorContext(ctx, "failed to create default event types", "error", err)
	}

	newAPIKey := &datastore.APIKey{
		Name: fmt.Sprintf("%s's default key", project.Name),
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
	}

	cak := CreateAPIKeyService{
		ProjectRepo: ps.ProjectRepo,
		APIKeyRepo:  ps.ApiKeyRepo,
		Member:      member,
		NewApiKey:   newAPIKey,
	}

	apiKey, keyString, err := cak.Run(ctx)
	if err != nil {
		return nil, nil, err
	}

	resp := &datastore.APIKeyResponse{
		APIKeyRes: datastore.APIKeyRes{
			Name: apiKey.Name,
			Role: datastore.Role{
				Type:    apiKey.Role.Type,
				Project: apiKey.Role.Project,
			},
			Type:      string(apiKey.Type),
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
		err := validateMetaEvent(project.Config, ps.Licenser)
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

	err := ps.ProjectRepo.UpdateProject(ctx, project)
	if err != nil {
		ps.Logger.ErrorContext(ctx, "failed to to update project", "error", err)
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

func validateMetaEvent(c *datastore.ProjectConfig, licenser license.Licenser) error {
	metaEvent := c.MetaEvent
	if metaEvent == nil {
		return nil
	}

	if !metaEvent.IsEnabled {
		return nil
	}

	if metaEvent.Type == datastore.HTTPMetaEvent {
		url, err := util.ValidateEndpoint(metaEvent.URL, c.SSL.EnforceSecureEndpoints, licenser.CustomCertificateAuthority())
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
