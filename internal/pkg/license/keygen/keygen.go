package keygen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/google/uuid"

	"github.com/frain-dev/convoy/datastore"
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
	keygen.Account = "1fddcec8-8dd3-4d8d-9b16-215cac0f9b52"
	keygen.Product = "1f086ec9-a943-46ea-9da4-e62c2180c2f4"
	keygen.PublicKey = "e8601e48b69383ba520245fd07971e983d06d22c4257cfd82304601479cee788"
}

func NewKeygenLicenser(c *Config) (*KeygenLicenser, error) {
	keygen.LicenseKey = c.LicenseKey

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	fingerprint := uuid.New().String()

	l, err := keygen.Validate(ctx, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("failed to validate error: %v", err)
	}

	if l.Metadata == nil {
		return nil, fmt.Errorf("license has no metadata")
	}

	featureList, err := getFeatureList(l)
	if err != nil {
		return nil, err
	}

	p := l.Metadata["plan_type"]
	if p == nil {
		return nil, fmt.Errorf("nil license metadata")
	}

	pt, ok := p.(string)
	if !ok {
		return nil, fmt.Errorf("license plan type is not a string")
	}

	planType := PlanType(pt)
	if !planType.IsValid() {
		return nil, fmt.Errorf("license plan type is not valid: %s", planType)
	}

	return &KeygenLicenser{
		machineFingerprint: fingerprint,
		licenseKey:         c.LicenseKey,
		license:            l,
		orgRepo:            c.OrgRepo,
		orgMemberRepo:      c.OrgMemberRepo,
		planType:           planType,
		featureList:        featureList,
	}, nil
}

func (k *KeygenLicenser) Activate() error {
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

var (
	ErrNoFeatureList  = errors.New("license has no feature list")
	ErrUnexpectedType = errors.New("license feature list has unexpected type")
)

func getFeatureList(l *keygen.License) (map[Feature]Properties, error) {
	m := l.Metadata["features"]
	if m == nil {
		return nil, ErrNoFeatureList
	}

	v, ok := m.(map[string]interface{})
	if !ok {
		return nil, ErrUnexpectedType
	}

	featureList := map[Feature]Properties{}
	err := mapstructure.Decode(v, &featureList)
	return featureList, err
}

func (k *KeygenLicenser) CanCreateOrg(ctx context.Context) (bool, error) {
	c, err := k.orgRepo.CountOrganisations(ctx)
	if err != nil {
		return false, err
	}

	p := k.featureList[CreateOrg]
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
