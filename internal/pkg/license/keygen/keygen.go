package keygen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"

	"github.com/google/uuid"
	"github.com/keygen-sh/keygen-go/v3"
)

type KeygenLicenser struct {
	licenseKey         string
	license            *keygen.License
	planType           PlanType
	machineFingerprint string
	featureList        map[Feature]Properties

	orgRepo       datastore.OrganisationRepository
	orgMemberRepo datastore.OrganisationMemberRepository
}

type Config struct {
	LicenseKey    string
	OrgRepo       datastore.OrganisationRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
}

func init() {
	keygen.Account = "cb3ba418-a9a0-4aa1-8c20-a844a06eeac9"
	keygen.Product = "c1374f6f-cdca-4a96-8d37-12a0ab8da241"
	keygen.PublicKey = "a64878b9361988b6943a9a93a0d4dd4056dfbe511da257ed0cf1476be8c0c34e"
}

func NewKeygenLicenser(c *Config) (*KeygenLicenser, error) {
	if util.IsStringEmpty(c.LicenseKey) {
		// no license key provided, allow access to only community features
		return communityLicenser(c.OrgRepo, c.OrgMemberRepo), nil
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

	featureList, err := getFeatureList(l)
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

	return &KeygenLicenser{
		machineFingerprint: fingerprint,
		licenseKey:         c.LicenseKey,
		license:            l,
		orgRepo:            c.OrgRepo,
		orgMemberRepo:      c.OrgMemberRepo,
		planType:           PlanType(pt),
		featureList:        featureList,
	}, nil
}

func (k *KeygenLicenser) Activate() error {
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
		// heartbeat monitor hrlps to tell keygen that this machin should be deactivated
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

var (
	ErrNoFeatureList  = errors.New("license has no feature list")
	ErrUnexpectedType = errors.New("license feature list has unexpected type")
)

func getFeatureList(l *keygen.License) (map[Feature]Properties, error) {
	m := l.Metadata["features"]
	if m == nil {
		return nil, ErrNoFeatureList
	}

	v, ok := m.(string)
	if !ok {
		return nil, ErrUnexpectedType
	}

	featureList := map[Feature]Properties{}
	err := json.Unmarshal([]byte(v), &featureList)
	return featureList, err
}

func (k *KeygenLicenser) CanCreateOrg(ctx context.Context) (bool, error) {
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

func (k *KeygenLicenser) CanCreateOrgMember(ctx context.Context) (bool, error) {
	c, err := k.orgMemberRepo.CountOrganisationMembers(ctx)
	if err != nil {
		return false, err
	}

	p := k.featureList[CreateOrgMember]

	if p.Limit == -1 { // no limit
		return true, nil
	}

	if c >= p.Limit {
		return false, nil
	}

	return true, nil
}

func (k *KeygenLicenser) CanUseForwardProxy() bool {
	_, ok := k.featureList[UseForwardProxy]
	return ok
}

func (k *KeygenLicenser) CanExportPrometheusMetrics() bool {
	_, ok := k.featureList[ExportPrometheusMetrics]
	return ok
}

func (k *KeygenLicenser) AdvancedEndpointMgmt() bool {
	_, ok := k.featureList[AdvancedEndpointMgmt]
	return ok
}

func (k *KeygenLicenser) AdvancedRetentionPolicy() bool {
	_, ok := k.featureList[AdvancedRetentionPolicy]
	return ok
}

func (k *KeygenLicenser) AdvancedMsgBroker() bool {
	_, ok := k.featureList[AdvancedMsgBroker]
	return ok
}

func (k *KeygenLicenser) AdvancedSubscriptions() bool {
	_, ok := k.featureList[AdvancedSubscriptions]
	return ok
}

func (k *KeygenLicenser) Transformations() bool {
	_, ok := k.featureList[Transformations]
	return ok
}

func (k *KeygenLicenser) HADeployment() bool {
	_, ok := k.featureList[HADeployment]
	return ok
}

func (k *KeygenLicenser) WebhookAnalytics() bool {
	_, ok := k.featureList[WebhookAnalytics]
	return ok
}

func (k *KeygenLicenser) MutualTLS() bool {
	_, ok := k.featureList[MutualTLS]
	return ok
}

func (k *KeygenLicenser) AsynqMonitoring() bool {
	_, ok := k.featureList[AsynqMonitoring]
	return ok
}

func (k *KeygenLicenser) SynchronousWebhooks() bool {
	_, ok := k.featureList[SynchronousWebhooks]
	return ok
}
