package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/auth"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProjectService struct {
	apiKeyRepo        datastore.APIKeyRepository
	projectRepo       datastore.ProjectRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	limiter           limiter.RateLimiter
	cache             cache.Cache
}

func NewProjectService(apiKeyRepo datastore.APIKeyRepository, projectRepo datastore.ProjectRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, limiter limiter.RateLimiter, cache cache.Cache) *ProjectService {
	return &ProjectService{
		apiKeyRepo:        apiKeyRepo,
		projectRepo:       projectRepo,
		eventRepo:         eventRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		limiter:           limiter,
		cache:             cache,
	}
}

func (ps *ProjectService) CreateProject(ctx context.Context, newProject *models.Project, org *datastore.Organisation, member *datastore.OrganisationMember) (*datastore.Project, *models.APIKeyResponse, error) {
	err := util.Validate(newProject)
	if err != nil {
		return nil, nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	projectName := newProject.Name

	config := newProject.Config
	if newProject.Config == nil {
		config = &datastore.ProjectConfig{}
		config.Signature = datastore.GetDefaultSignatureConfig()
		config.Strategy = &datastore.DefaultStrategyConfig
		config.RateLimit = &datastore.DefaultRateLimitConfig
		config.RetentionPolicy = &datastore.DefaultRetentionPolicy
	} else {
		if newProject.Config.Signature == nil {
			config.Signature = datastore.GetDefaultSignatureConfig()
		}

		checkSignatureVersions(newProject.Config.Signature.Versions)

		if newProject.Config.Strategy == nil {
			config.Strategy = &datastore.DefaultStrategyConfig
		}

		if newProject.Config.RateLimit == nil {
			config.RateLimit = &datastore.DefaultRateLimitConfig
		}

		if newProject.Config.RetentionPolicy == nil {
			config.RetentionPolicy = &datastore.DefaultRetentionPolicy
		}

	}

	if newProject.RateLimit == 0 {
		newProject.RateLimit = convoy.RATE_LIMIT
	}

	if util.IsStringEmpty(newProject.RateLimitDuration) {
		newProject.RateLimitDuration = convoy.RATE_LIMIT_DURATION
	}

	project := &datastore.Project{
		UID:               uuid.New().String(),
		Name:              projectName,
		Type:              newProject.Type,
		OrganisationID:    org.UID,
		Config:            config,
		LogoURL:           newProject.LogoURL,
		CreatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		RateLimit:         newProject.RateLimit,
		RateLimitDuration: newProject.RateLimitDuration,
	}

	err = ps.projectRepo.CreateProject(ctx, project)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create project")
		if err == datastore.ErrDuplicateProjectName {
			return nil, nil, util.NewServiceError(http.StatusBadRequest, err)
		}

		return nil, nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create project"))
	}

	newAPIKey := &models.APIKey{
		Name: fmt.Sprintf("%s's default key", project.Name),
		Role: models.Role{
			Type:    auth.RoleAdmin,
			Project: project.UID,
		},
	}

	apiKey, keyString, err := NewSecurityService(ps.projectRepo, ps.apiKeyRepo).CreateAPIKey(ctx, member, newAPIKey)
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
			ExpiresAt: apiKey.ExpiresAt.Time(),
		},
		UID:       apiKey.UID,
		CreatedAt: apiKey.CreatedAt.Time(),
		Key:       keyString,
	}

	return project, resp, nil
}

func (ps *ProjectService) UpdateProject(ctx context.Context, project *datastore.Project, update *models.UpdateProject) (*datastore.Project, error) {
	err := util.Validate(update)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to validate project update")
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if !util.IsStringEmpty(update.Name) {
		project.Name = update.Name
	}

	if update.Config != nil {
		project.Config = update.Config
		checkSignatureVersions(project.Config.Signature.Versions)
	}

	if !util.IsStringEmpty(update.LogoURL) {
		project.LogoURL = update.LogoURL
	}

	err = ps.projectRepo.UpdateProject(ctx, project)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to to update project")
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	projectCacheKey := convoy.ProjectsCacheKey.Get(project.UID).String()
	err = ps.cache.Set(ctx, projectCacheKey, &project, time.Minute*5)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	return project, nil
}

func checkSignatureVersions(versions []datastore.SignatureVersion) {
	for i := range versions {
		v := &versions[i]
		if v.UID == "" {
			v.UID = uuid.NewString()
		}

		if v.CreatedAt == 0 {
			v.CreatedAt = primitive.NewDateTimeFromTime(time.Now())
		}
	}
}

func (ps *ProjectService) GetProjects(ctx context.Context, filter *datastore.ProjectFilter) ([]*datastore.Project, error) {
	projects, err := ps.projectRepo.LoadProjects(ctx, filter.WithNamesTrimmed())
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to load projects")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while fetching projects"))
	}

	err = ps.FillProjectStatistics(ctx, projects)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fill statistics of project")
	}

	return projects, nil
}

func (ps *ProjectService) FillProjectStatistics(ctx context.Context, projects []*datastore.Project) error {
	err := ps.projectRepo.FillProjectsStatistics(ctx, projects)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to count project applications")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to count project statistics"))
	}

	return nil
}

func (ps *ProjectService) DeleteProject(ctx context.Context, id string) error {
	err := ps.projectRepo.DeleteProject(ctx, id)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to delete project")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to delete project"))
	}

	return nil
}
