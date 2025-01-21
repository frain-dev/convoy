package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/instance"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/license"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
	"gopkg.in/guregu/null.v4"
)

type CreateOrganisationService struct {
	OrgRepo               datastore.OrganisationRepository
	OrgMemberRepo         datastore.OrganisationMemberRepository
	InstanceOverridesRepo datastore.InstanceOverridesRepository
	NewOrg                *models.Organisation
	User                  *datastore.User
	Licenser              license.Licenser
	RoleType              auth.RoleType
}

var ErrOrgLimit = errors.New("your instance has reached it's organisation limit, upgrade to create new organisations")

func (co *CreateOrganisationService) Run(ctx context.Context) (*datastore.Organisation, error) {
	ok, err := co.Licenser.CreateOrg(ctx)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if !ok {
		return nil, &ServiceError{ErrMsg: ErrOrgLimit.Error(), Err: ErrOrgLimit}
	}

	err = util.Validate(co.NewOrg)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if len(co.NewOrg.Name) == 0 {
		log.FromContext(ctx).WithError(err).Error("organisation name is required")
		return nil, &ServiceError{ErrMsg: "organisation name is required", Err: err}
	}

	if co.RoleType == "" {
		co.RoleType = auth.RoleOrganisationAdmin
	}

	org := &datastore.Organisation{
		UID:       ulid.Make().String(),
		OwnerID:   co.User.UID,
		Name:      co.NewOrg.Name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	cfg, err := config.Get()
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to load config")
		return nil, &ServiceError{ErrMsg: "failed to create organisation", Err: err}
	}

	if len(cfg.CustomDomainSuffix) > 0 {
		org.AssignedDomain = null.NewString(fmt.Sprintf("%s.%s", uniuri.New(), cfg.CustomDomainSuffix), true)
	}

	err = co.OrgRepo.CreateOrganisation(ctx, org)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create organisation")
		return nil, &ServiceError{ErrMsg: "failed to create organisation", Err: err}
	}
	_, err = NewOrganisationMemberService(co.OrgMemberRepo, co.Licenser).CreateOrganisationMember(ctx, org, co.User, &auth.Role{Type: co.RoleType})
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create super_user member for organisation owner")
	}

	err = UpdateInstanceConfig(ctx, co.InstanceOverridesRepo, org, co.NewOrg)
	if err != nil {
		return nil, err
	}

	return org, nil
}

func UpdateInstanceConfig(ctx context.Context, repo datastore.InstanceOverridesRepository, org *datastore.Organisation, newOrg *models.Organisation) error {
	orgCfg := newOrg.Config
	if orgCfg != nil {
		if org.Config == nil {
			org.Config = &datastore.InstanceConfig{}
		}
		if org.Config.ProjectConfig == nil {
			org.Config.ProjectConfig = &datastore.ProjectInstanceConfig{}
		}

		keysToUpdate := map[string]bool{}

		org.Config.StaticIP = nil
		if orgCfg.StaticIP != nil {
			boolean := instance.Boolean{Value: *orgCfg.StaticIP}
			bytes, err := json.Marshal(boolean)
			if err != nil {
				return err
			}

			if err := handleOptionalJSONField(ctx, org, repo, instance.KeyStaticIP, string(bytes)); err != nil {
				return err
			}
			org.Config.StaticIP = orgCfg.StaticIP
			keysToUpdate[instance.KeyStaticIP] = true
		}

		org.Config.EnterpriseSSO = nil
		if orgCfg.EnterpriseSSO != nil {
			boolean := instance.Boolean{Value: *orgCfg.EnterpriseSSO}
			bytes, err := json.Marshal(boolean)
			if err != nil {
				return err
			}

			if err := handleOptionalJSONField(ctx, org, repo, instance.KeyEnterpriseSSO, string(bytes)); err != nil {
				return err
			}
			org.Config.EnterpriseSSO = orgCfg.EnterpriseSSO
			keysToUpdate[instance.KeyEnterpriseSSO] = true
		}

		org.Config.ProjectConfig.RetentionPolicy = nil
		if orgCfg.ProjectConfig != nil && orgCfg.ProjectConfig.RetentionPolicy != nil && orgCfg.ProjectConfig.RetentionPolicy.Policy != "" {
			policy := orgCfg.ProjectConfig.RetentionPolicy.Policy
			_, err := time.ParseDuration(policy)
			if err != nil {
				return err
			}
			bytes, err := json.Marshal(orgCfg.ProjectConfig.RetentionPolicy)
			if err != nil {
				return err
			}

			if err := handleOptionalJSONField(ctx, org, repo, instance.KeyRetentionPolicy, string(bytes)); err != nil {
				return err
			}
			if org.Config.ProjectConfig.RetentionPolicy == nil {
				org.Config.ProjectConfig.RetentionPolicy = &config.RetentionPolicyConfiguration{}
			}
			org.Config.ProjectConfig.RetentionPolicy.Policy = orgCfg.ProjectConfig.RetentionPolicy.Policy
			keysToUpdate[instance.KeyRetentionPolicy] = true
		}

		org.Config.ProjectConfig.IngestRateLimit = nil
		if orgCfg.ProjectConfig != nil && orgCfg.ProjectConfig.IngestRateLimit != nil {
			rateLimit := orgCfg.ProjectConfig.IngestRateLimit
			if *rateLimit < 1 {
				return fmt.Errorf("ingest rate limit must be greater than or equal to 1: %v", rateLimit)
			}
			boolean := instance.IngestRate{Value: *orgCfg.ProjectConfig.IngestRateLimit}
			bytes, err := json.Marshal(boolean)
			if err != nil {
				return err
			}

			if err := handleOptionalJSONField(ctx, org, repo, instance.KeyInstanceIngestRate, string(bytes)); err != nil {
				return err
			}
			org.Config.ProjectConfig.IngestRateLimit = orgCfg.ProjectConfig.IngestRateLimit
			keysToUpdate[instance.KeyInstanceIngestRate] = true
		}

		err := deleteUnUpdatedKeys(ctx, repo, org.UID, keysToUpdate)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteUnUpdatedKeys(ctx context.Context, repo datastore.InstanceOverridesRepository, scopeID string, keysToUpdate map[string]bool) error {
	return repo.DeleteUnUpdatedKeys(ctx, instance.OrganisationScope, scopeID, keysToUpdate)
}

func handleOptionalJSONField(ctx context.Context, org *datastore.Organisation, repo datastore.InstanceOverridesRepository, key string, jsonValue string) error {
	_, err := repo.Create(ctx, &datastore.InstanceOverrides{
		ScopeType: instance.OrganisationScope,
		ScopeID:   org.UID,
		Key:       key,
		Value:     jsonValue,
	})
	return err
}
