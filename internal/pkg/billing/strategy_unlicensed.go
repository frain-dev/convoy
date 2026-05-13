package billing

import (
	"context"
	"net/http"

	"github.com/frain-dev/convoy/config"
)

type unlicensedStrategy struct {
	licensedStrategy
	resolveOwner OwnerEmailResolver
}

func (s *unlicensedStrategy) Mode() config.BillingMode { return config.BillingModeUnlicensed }

func (s *unlicensedStrategy) GetUsage(ctx context.Context, orgID string) (*Response[Usage], error) {
	return localUsage(ctx, s.orgRepo, orgID)
}

func (s *unlicensedStrategy) GetInvoices(ctx context.Context, orgID string) (*Response[[]Invoice], error) {
	return emptyResponse[[]Invoice]("Invoices retrieved successfully"), nil
}

func (s *unlicensedStrategy) GetInvoice(ctx context.Context, orgID, invoiceID string) (*Response[Invoice], error) {
	return nil, &ServiceError{StatusCode: http.StatusNotFound, Message: "no invoices for this organisation yet"}
}

func (s *unlicensedStrategy) DownloadInvoice(ctx context.Context, orgID, invoiceID string) (*http.Response, string, error) {
	return nil, "", &ServiceError{StatusCode: http.StatusNotFound, Message: "no invoices for this organisation yet"}
}

func (s *unlicensedStrategy) GetSubscription(ctx context.Context, orgID string) (*Response[BillingSubscription], error) {
	return &Response[BillingSubscription]{
		Status:  true,
		Message: "No active subscription",
		Data:    BillingSubscription{Status: "none"},
	}, nil
}

func (s *unlicensedStrategy) GetSubscriptions(ctx context.Context, orgID string) (*Response[[]BillingSubscription], error) {
	return emptyResponse[[]BillingSubscription]("No active subscriptions"), nil
}

func (s *unlicensedStrategy) GetPaymentMethods(ctx context.Context, orgID string) (*Response[[]PaymentMethod], error) {
	return emptyResponse[[]PaymentMethod]("No payment methods on file"), nil
}

func (s *unlicensedStrategy) GetSetupIntent(ctx context.Context, orgID string) (*Response[SetupIntent], error) {
	return nil, ErrNoLicense
}

func (s *unlicensedStrategy) GetPlans(ctx context.Context, orgID string) (*Response[[]Plan], error) {
	return s.client.GetPlans(ctx)
}

func (s *unlicensedStrategy) GetTaxIDTypes(ctx context.Context, orgID string) (*Response[[]TaxIDType], error) {
	return s.client.GetTaxIDTypes(ctx)
}

func (s *unlicensedStrategy) GetOrganisation(ctx context.Context, orgID string) (*Response[BillingOrganisation], error) {
	name := ""
	if s.orgRepo != nil {
		if org, err := s.orgRepo.FetchOrganisationByID(ctx, orgID); err == nil && org != nil {
			name = org.Name
		}
	}
	return &Response[BillingOrganisation]{
		Status: true,
		Data: BillingOrganisation{
			ExternalID: orgID,
			Name:       name,
		},
	}, nil
}

func (s *unlicensedStrategy) GetInternalOrganisationID(ctx context.Context, orgID string) (string, error) {
	return "", ErrNoLicense
}

func (s *unlicensedStrategy) LicenseSummary(ctx context.Context, orgID string) LicenseSummary {
	return LicenseSummary{}
}

func (s *unlicensedStrategy) OnboardSubscription(ctx context.Context, orgID string, req OnboardSubscriptionRequest) (*Response[Checkout], error) {
	return nil, ErrNoLicense
}

func (s *unlicensedStrategy) UpgradeSubscription(ctx context.Context, orgID, subscriptionID string, req UpgradeSubscriptionRequest) (*Response[Checkout], error) {
	return nil, ErrNoLicense
}

func (s *unlicensedStrategy) DeleteSubscription(ctx context.Context, orgID, subscriptionID string) (*Response[interface{}], error) {
	return nil, ErrNoLicense
}

func (s *unlicensedStrategy) SetDefaultPaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error) {
	return nil, ErrNoLicense
}

func (s *unlicensedStrategy) DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error) {
	return nil, ErrNoLicense
}

func (s *unlicensedStrategy) UpdateOrganisation(ctx context.Context, orgID string, data BillingOrganisation) (*Response[BillingOrganisation], error) {
	return nil, ErrNoLicense
}

func (s *unlicensedStrategy) UpdateOrganisationTaxID(ctx context.Context, orgID string, data UpdateOrganisationTaxIDRequest) (*Response[BillingOrganisation], error) {
	return nil, ErrNoLicense
}

func (s *unlicensedStrategy) UpdateOrganisationAddress(ctx context.Context, orgID string, data UpdateOrganisationAddressRequest) (*Response[BillingOrganisation], error) {
	return nil, ErrNoLicense
}
