package keygen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"

	"github.com/google/uuid"
	"github.com/keygen-sh/keygen-go/v3"
)

type Licenser struct {
	licenseKey         string
	license            *keygen.License
	planType           PlanType
	machineFingerprint string
	featureList        map[Feature]Properties
	featureListJSON    []byte

	orgRepo       datastore.OrganisationRepository
	orgMemberRepo datastore.OrganisationMemberRepository
	projectRepo   datastore.ProjectRepository
}

func (k *Licenser) FeatureListJSON() json.RawMessage {
	return k.featureListJSON
}

type Config struct {
	LicenseKey    string
	OrgRepo       datastore.OrganisationRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
	ProjectRepo   datastore.ProjectRepository
}

func init() {
	keygen.Account = "8200bc0f-f64f-4a38-a9be-d2b16c8f0deb"
	keygen.Product = "08d95b4d-4301-42f9-95af-9713e1b41a3a"
	keygen.PublicKey = "14549f18dd23e4644aae6b6fd787e4df5f018bce0c7ae2edd29df83309ea76c2"
}

func NewKeygenLicenser(c *Config) (*Licenser, error) {
	if util.IsStringEmpty(c.LicenseKey) {
		// no license key provided, allow access to only community features
		return communityLicenser(c.OrgRepo, c.OrgMemberRepo, c.ProjectRepo)
	}

	keygen.LicenseKey = c.LicenseKey

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	fingerprint := uuid.New().String()

	l, err := keygen.Validate(ctx, fingerprint)
	if err != nil && !allowKeygenError(err) {
		return nil, fmt.Errorf("failed to validate error: %v", err)
	}

	if l.Metadata == nil {
		return nil, fmt.Errorf("license has no metadata")
	}

	featureList, err := getFeatureList(ctx, l)
	if err != nil {
		return nil, err
	}

	p := l.Metadata["planType"]
	if p == nil {
		return nil, fmt.Errorf("license plan type unspecified in metadata")
	}

	pt, ok := p.(string)
	if !ok {
		return nil, fmt.Errorf("license plan type is not a string")
	}

	featureListJSON, err := json.Marshal(featureList)
	if err != nil {
		return nil, err
	}

	return &Licenser{
		machineFingerprint: fingerprint,
		licenseKey:         c.LicenseKey,
		license:            l,
		orgRepo:            c.OrgRepo,
		orgMemberRepo:      c.OrgMemberRepo,
		planType:           PlanType(pt),
		featureList:        featureList,
		featureListJSON:    featureListJSON,
	}, nil
}

func (k *Licenser) Activate() error {
	if util.IsStringEmpty(k.licenseKey) {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	machine, err := k.license.Activate(ctx, k.machineFingerprint)
	if err != nil {
		return fmt.Errorf("failed to activate machine")
	}

	// Start a heartbeat monitor for the current machine
	err = machine.Monitor(ctx)
	if err != nil {
		return fmt.Errorf("failed to start machine monitor")
	}

	go func() {
		// Listen for interrupt and deactivate the machine, if the instance crashes unexpectedly the
		// heartbeat monitor helps to tell keygen that this machine should be deactivated
		// See the Check-out/check-in licenses section on
		// https://keygen.sh/docs/choosing-a-licensing-model/floating-licenses/
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt)
		<-quit

		if err := machine.Deactivate(ctx); err != nil {
			log.WithError(err).Error("failed to deactivate machine")
		}
	}()

	return nil
}

func allowKeygenError(err error) bool {
	switch {
	case errors.Is(err, keygen.ErrLicenseNotActivated):
		return true
	case errors.Is(err, keygen.ErrHeartbeatRequired):
		return true
	}

	return false
}

func getFeatureList(ctx context.Context, l *keygen.License) (map[Feature]Properties, error) {
	entitlements, err := l.Entitlements(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load license entitlements: %v", err)
	}

	if len(entitlements) == 0 {
		return nil, fmt.Errorf("license has no entitlements")
	}

	featureList := map[Feature]Properties{}
	for _, entitlement := range entitlements {
		featureList[Feature(entitlement.Code)] = Properties{}
	}

	meta := LicenseMetadata{}
	if l.Metadata != nil {
		err = mapstructure.Decode(l.Metadata, &meta)
		if err != nil {
			return nil, fmt.Errorf("failed to decode license metadata: %v", err)
		}
	}

	if meta.OrgLimit != 0 {
		featureList[CreateOrg] = Properties{Limit: meta.OrgLimit}
	}

	if meta.UserLimit != 0 {
		featureList[CreateUser] = Properties{Limit: meta.UserLimit}
	}

	if meta.ProjectLimit != 0 {
		featureList[CreateProject] = Properties{Limit: meta.ProjectLimit}
	}

	return featureList, err
}

func (k *Licenser) CreateOrg(ctx context.Context) (bool, error) {
	c, err := k.orgRepo.CountOrganisations(ctx)
	if err != nil {
		return false, err
	}

	p := k.featureList[CreateOrg]

	if p.Limit == -1 { // no limit
		return true, nil
	}

	if c >= p.Limit {
		return false, nil
	}

	return true, nil
}

func (k *Licenser) CreateUser(ctx context.Context) (bool, error) {
	c, err := k.orgMemberRepo.CountOrganisationMembers(ctx)
	if err != nil {
		return false, err
	}

	p := k.featureList[CreateUser]

	if p.Limit == -1 { // no limit
		return true, nil
	}

	if c >= p.Limit {
		return false, nil
	}

	return true, nil
}

func (k *Licenser) CreateProject(ctx context.Context) (bool, error) {
	c, err := k.projectRepo.CountProjects(ctx)
	if err != nil {
		return false, err
	}

	p := k.featureList[CreateProject]

	if p.Limit == -1 { // no limit
		return true, nil
	}

	if c >= p.Limit {
		return false, nil
	}

	return true, nil
}

func (k *Licenser) UseForwardProxy() bool {
	_, ok := k.featureList[UseForwardProxy]
	return ok
}

func (k *Licenser) CanExportPrometheusMetrics() bool {
	_, ok := k.featureList[ExportPrometheusMetrics]
	return ok
}

func (k *Licenser) AdvancedEndpointMgmt() bool {
	_, ok := k.featureList[AdvancedEndpointMgmt]
	return ok
}

func (k *Licenser) AdvancedRetentionPolicy() bool {
	_, ok := k.featureList[AdvancedWebhookArchiving]
	return ok
}

func (k *Licenser) AdvancedMsgBroker() bool {
	_, ok := k.featureList[AdvancedMsgBroker]
	return ok
}

func (k *Licenser) AdvancedSubscriptions() bool {
	_, ok := k.featureList[AdvancedSubscriptions]
	return ok
}

func (k *Licenser) Transformations() bool {
	_, ok := k.featureList[WebhookTransformations]
	return ok
}

func (k *Licenser) HADeployment() bool {
	_, ok := k.featureList[HADeployment]
	return ok
}

func (k *Licenser) WebhookAnalytics() bool {
	_, ok := k.featureList[WebhookAnalytics]
	return ok
}

func (k *Licenser) MutualTLS() bool {
	_, ok := k.featureList[MutualTLS]
	return ok
}

func (k *Licenser) AsynqMonitoring() bool {
	_, ok := k.featureList[AsynqMonitoring]
	return ok
}

func (k *Licenser) SynchronousWebhooks() bool {
	_, ok := k.featureList[SynchronousWebhooks]
	return ok
}
