package billing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/logger"
)

type Strategy interface {
	Mode() config.BillingMode

	// Reads
	GetUsage(ctx context.Context, orgID string) (*Response[Usage], error)
	GetInvoices(ctx context.Context, orgID string) (*Response[[]Invoice], error)
	GetInvoice(ctx context.Context, orgID, invoiceID string) (*Response[Invoice], error)
	DownloadInvoice(ctx context.Context, orgID, invoiceID string) (*http.Response, string, error)
	GetSubscription(ctx context.Context, orgID string) (*Response[BillingSubscription], error)
	GetSubscriptions(ctx context.Context, orgID string) (*Response[[]BillingSubscription], error)
	GetPaymentMethods(ctx context.Context, orgID string) (*Response[[]PaymentMethod], error)
	GetSetupIntent(ctx context.Context, orgID string) (*Response[SetupIntent], error)
	GetPlans(ctx context.Context, orgID string) (*Response[[]Plan], error)
	GetTaxIDTypes(ctx context.Context, orgID string) (*Response[[]TaxIDType], error)
	GetOrganisation(ctx context.Context, orgID string) (*Response[BillingOrganisation], error)
	GetInternalOrganisationID(ctx context.Context, orgID string) (string, error)
	LicenseSummary(ctx context.Context, orgID string) LicenseSummary

	// Writes
	SelfHostedRegisterEmail(ctx context.Context, req SelfHostedRegisterEmailRequest) (*Response[SelfHostedRegisterEmailData], error)
	SelfHostedVerifyEmail(ctx context.Context, code string) (*Response[SelfHostedVerifyEmailData], error)
	CreateOrganisation(ctx context.Context, data BillingOrganisation) (*Response[BillingOrganisation], error)
	OnboardSubscription(ctx context.Context, orgID string, req OnboardSubscriptionRequest) (*Response[Checkout], error)
	UpgradeSubscription(ctx context.Context, orgID, subscriptionID string, req UpgradeSubscriptionRequest) (*Response[Checkout], error)
	DeleteSubscription(ctx context.Context, orgID, subscriptionID string) (*Response[interface{}], error)
	SetDefaultPaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error)
	DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error)
	UpdateOrganisation(ctx context.Context, orgID string, data BillingOrganisation) (*Response[BillingOrganisation], error)
	UpdateOrganisationTaxID(ctx context.Context, orgID string, data UpdateOrganisationTaxIDRequest) (*Response[BillingOrganisation], error)
	UpdateOrganisationAddress(ctx context.Context, orgID string, data UpdateOrganisationAddressRequest) (*Response[BillingOrganisation], error)
}

// SubscriptionStatusReader is the Strategy surface used for org-level subscription gating.
type SubscriptionStatusReader interface {
	GetSubscription(ctx context.Context, orgID string) (*Response[BillingSubscription], error)
}

type LicenseSummary struct {
	Configured      bool   `json:"configured"`
	MaskedKey       string `json:"masked_key"`
	HasEntitlements bool   `json:"has_entitlements"`
}

var ErrNoLicense = errors.New("no license configured for this organisation")

func NewStrategy(cfg config.Configuration, client Client, log logger.Logger, orgRepo datastore.OrganisationRepository, resolveOwner OwnerEmailResolver) Strategy {
	switch cfg.Mode() {
	case config.BillingModeCloud:
		return &cloudStrategy{
			client:       client,
			logger:       log,
			orgRepo:      orgRepo,
			host:         cfg.Host,
			resolveOwner: resolveOwner,
		}
	case config.BillingModeLicensed:
		return &licensedStrategy{
			client:             client,
			logger:             log,
			orgRepo:            orgRepo,
			instanceLicenseKey: cfg.LicenseKey,
		}
	default:
		return &unlicensedStrategy{
			licensedStrategy: licensedStrategy{
				client:             client,
				logger:             log,
				orgRepo:            orgRepo,
				instanceLicenseKey: cfg.LicenseKey,
			},
			resolveOwner: resolveOwner,
		}
	}
}

const PostPurchaseInstructionsForUnlicensed = "Set the issued key as CONVOY_LICENSE_KEY and restart Convoy."

func MaskLicenseKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	runes := []rune(key)
	for i := 4; i < len(runes)-4; i++ {
		if runes[i] != '-' {
			runes[i] = '*'
		}
	}
	return string(runes)
}

// selfHostedLicenseBillingKey is the Overwatch license-billing credential for self-hosted
// strategies (licensed / unlicensed). It is only CONVOY_LICENSE_KEY; organisation license_data
// is not consulted for these HTTP calls.
func selfHostedLicenseBillingKey(instanceKey string) (string, error) {
	k := strings.TrimSpace(instanceKey)
	if k == "" {
		return "", ErrNoLicense
	}
	return k, nil
}

func emptyResponse[T any](message string) *Response[T] {
	var zero T
	return &Response[T]{Status: true, Message: message, Data: zero}
}

func monthBoundsUTC(now time.Time) (time.Time, time.Time) {
	now = now.UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Nanosecond)
	return startOfMonth, endOfMonth
}

func localUsage(ctx context.Context, orgRepo datastore.OrganisationRepository, orgID string) (*Response[Usage], error) {
	if orgRepo == nil {
		return nil, fmt.Errorf("organisation repository is required to compute usage")
	}
	start, end := monthBoundsUTC(time.Now())
	usage, err := orgRepo.CalculateUsage(ctx, orgID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate usage: %w", err)
	}
	return &Response[Usage]{
		Status:  true,
		Message: "Usage retrieved successfully",
		Data: Usage{
			OrganisationID: usage.OrganisationID,
			Period:         usage.Period,
			Received:       UsageMetrics{Volume: usage.Received.Volume, Bytes: usage.Received.Bytes},
			Sent:           UsageMetrics{Volume: usage.Sent.Volume, Bytes: usage.Sent.Bytes},
			CreatedAt:      usage.CreatedAt.Format(time.RFC3339),
		},
	}, nil
}
