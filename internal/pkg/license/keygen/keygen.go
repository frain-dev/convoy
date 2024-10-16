package keygen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
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
	featureList        map[Feature]*Properties

	orgRepo     datastore.OrganisationRepository
	userRepo    datastore.UserRepository
	projectRepo datastore.ProjectRepository

	// only for community licenser
	mu              sync.RWMutex
	enabledProjects map[string]bool
}

type Config struct {
	LicenseKey  string
	OrgRepo     datastore.OrganisationRepository
	ProjectRepo datastore.ProjectRepository
	UserRepo    datastore.UserRepository
}

func init() {
	keygen.Account = "8200bc0f-f64f-4a38-a9be-d2b16c8f0deb"
	keygen.Product = "08d95b4d-4301-42f9-95af-9713e1b41a3a"
	keygen.PublicKey = "14549f18dd23e4644aae6b6fd787e4df5f018bce0c7ae2edd29df83309ea76c2"
}

func NewKeygenLicenser(c *Config) (*Licenser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if util.IsStringEmpty(c.LicenseKey) {
		// no license key provided, allow access to only community features
		return communityLicenser(ctx, c.OrgRepo, c.UserRepo, c.ProjectRepo)
	}

	keygen.LicenseKey = c.LicenseKey
	fingerprint := uuid.New().String()

	l, err := keygen.Validate(ctx, fingerprint)
	if err != nil && !allowKeygenError(err) {
		return nil, fmt.Errorf("failed to validate error: %v", err)
	}

	err = checkExpiry(l)
	if err != nil {
		return nil, err
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

	return &Licenser{
		machineFingerprint: fingerprint,
		licenseKey:         c.LicenseKey,
		license:            l,
		orgRepo:            c.OrgRepo,
		userRepo:           c.UserRepo,
		projectRepo:        c.ProjectRepo,
		planType:           PlanType(pt),
		featureList:        featureList,
	}, err
}

func (k *Licenser) ProjectEnabled(projectID string) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.enabledProjects == nil { // not community licenser
		return true
	}

	return k.enabledProjects[projectID]
}

func (k *Licenser) AddEnabledProject(projectID string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.enabledProjects == nil { // not community licenser
		return
	}

	if len(k.enabledProjects) == projectLimit {
		return
	}

	k.enabledProjects[projectID] = true
}

func (k *Licenser) RemoveEnabledProject(projectID string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.enabledProjects == nil { // not community licenser
		return
	}

	delete(k.enabledProjects, projectID)
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
	case errors.Is(err, keygen.ErrLicenseExpired):
		return true
	case errors.Is(err, keygen.ErrHeartbeatRequired):
		return true
	}

	return false
}

var ErrLicenseExpired = errors.New("license expired")

func checkExpiry(l *keygen.License) error {
	if l.Expiry == nil {
		return nil
	}

	now := time.Now()

	if now.After(*l.Expiry) {
		v := now.Sub(*l.Expiry)

		const days = 21 * 24 * time.Hour // 21 days

		if v < days { // expired in less than 21 days, allow instance to boot
			daysAgo := int64(v.Hours() / 24)
			log.Warnf("license expired %d days ago, access to features will be revoked in %d days", daysAgo, 21-daysAgo)
			return nil
		}

		return ErrLicenseExpired
	}

	return nil
}

func getFeatureList(ctx context.Context, l *keygen.License) (map[Feature]*Properties, error) {
	entitlements, err := l.Entitlements(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load license entitlements: %v", err)
	}

	if len(entitlements) == 0 {
		return nil, fmt.Errorf("license has no entitlements")
	}

	featureList := map[Feature]*Properties{}
	for _, entitlement := range entitlements {
		featureList[Feature(entitlement.Code)] = &Properties{Allowed: true}
	}

	meta := LicenseMetadata{}
	if l.Metadata != nil {
		err = mapstructure.Decode(l.Metadata, &meta)
		if err != nil {
			return nil, fmt.Errorf("failed to decode license metadata: %v", err)
		}
	}

	if meta.OrgLimit != 0 {
		featureList[CreateOrg] = &Properties{Limit: meta.OrgLimit}
	}

	if meta.UserLimit != 0 {
		featureList[CreateUser] = &Properties{Limit: meta.UserLimit}
	}

	if meta.ProjectLimit != 0 {
		featureList[CreateProject] = &Properties{Limit: meta.ProjectLimit}
	}

	return featureList, err
}

func (k *Licenser) CreateOrg(ctx context.Context) (bool, error) {
	err := checkExpiry(k.license)
	if err != nil {
		return false, err
	}

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
	err := checkExpiry(k.license)
	if err != nil {
		return false, err
	}

	c, err := k.userRepo.CountUsers(ctx)
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
	err := checkExpiry(k.license)
	if err != nil {
		return false, err
	}

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
	if checkExpiry(k.license) != nil {
		return false
	}

	_, ok := k.featureList[UseForwardProxy]
	return ok
}

func (k *Licenser) CanExportPrometheusMetrics() bool {
	if checkExpiry(k.license) != nil {
		return false
	}

	_, ok := k.featureList[ExportPrometheusMetrics]
	return ok
}

func (k *Licenser) AdvancedEndpointMgmt() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[AdvancedEndpointMgmt]
	return ok
}

func (k *Licenser) AdvancedRetentionPolicy() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[AdvancedWebhookArchiving]
	return ok
}

func (k *Licenser) AdvancedMsgBroker() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[AdvancedMsgBroker]
	return ok
}

func (k *Licenser) AdvancedSubscriptions() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[AdvancedSubscriptions]
	return ok
}

func (k *Licenser) Transformations() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[WebhookTransformations]
	return ok
}

func (k *Licenser) HADeployment() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[HADeployment]
	return ok
}

func (k *Licenser) WebhookAnalytics() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[WebhookAnalytics]
	return ok
}

func (k *Licenser) MutualTLS() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[MutualTLS]
	return ok
}

func (k *Licenser) AsynqMonitoring() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[AsynqMonitoring]
	return ok
}

func (k *Licenser) SynchronousWebhooks() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[SynchronousWebhooks]
	return ok
}

func (k *Licenser) PortalLinks() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[PortalLinks]
	return ok
}

func (k *Licenser) ConsumerPoolTuning() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[ConsumerPoolTuning]
	return ok
}

func (k *Licenser) AdvancedWebhookFiltering() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[AdvancedWebhookFiltering]
	return ok
}

func (k *Licenser) CircuitBreaking() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[CircuitBreaking]
	return ok
}

func (k *Licenser) MultiPlayerMode() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[MultiPlayerMode]
	return ok
}

func (k *Licenser) IngestRate() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[IngestRate]
	return ok
}

func (k *Licenser) AgentExecutionMode() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[AgentExecutionMode]
	return ok
}

func (k *Licenser) IpRules() bool {
	if checkExpiry(k.license) != nil {
		return false
	}
	_, ok := k.featureList[IpRules]
	return ok
}

func (k *Licenser) FeatureListJSON(ctx context.Context) (json.RawMessage, error) {
	// only these guys have dynamic limits for now
	for f := range k.featureList {
		switch f {
		case CreateOrg:
			ok, err := k.CreateOrg(ctx)
			if err != nil {
				return nil, err
			}
			k.featureList[f].Allowed = ok
		case CreateUser:
			ok, err := k.CreateUser(ctx)
			if err != nil {
				return nil, err
			}
			k.featureList[f].Allowed = ok
		case CreateProject:
			ok, err := k.CreateProject(ctx)
			if err != nil {
				return nil, err
			}
			k.featureList[f].Allowed = ok
		}
	}

	return json.Marshal(k.featureList)
}
