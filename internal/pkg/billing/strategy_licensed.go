package billing

import (
	"context"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/logger"
)

type licensedStrategy struct {
	client             Client
	logger             logger.Logger
	orgRepo            datastore.OrganisationRepository
	instanceLicenseKey string
}

func (s *licensedStrategy) Mode() config.BillingMode { return config.BillingModeLicensed }

func (s *licensedStrategy) licenseKeyFor(ctx context.Context, orgID string) (string, error) {
	return resolveOrgLicenseKey(ctx, s.orgRepo, s.instanceLicenseKey, orgID)
}

func (s *licensedStrategy) GetUsage(ctx context.Context, orgID string) (*Response[Usage], error) {
	return localUsage(ctx, s.orgRepo, orgID)
}

func (s *licensedStrategy) GetInvoices(ctx context.Context, orgID string) (*Response[[]Invoice], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.LicenseBillingGetInvoices(ctx, lk)
}

func (s *licensedStrategy) GetInvoice(ctx context.Context, orgID, invoiceID string) (*Response[Invoice], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.LicenseBillingGetInvoice(ctx, lk, invoiceID)
}

func (s *licensedStrategy) DownloadInvoice(ctx context.Context, orgID, invoiceID string) (*http.Response, string, error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, "", err
	}
	inv, err := s.client.LicenseBillingGetInvoice(ctx, lk, invoiceID)
	if err != nil {
		return nil, "", err
	}
	if inv.Data.PDFLink == "" {
		return nil, "", &ServiceError{StatusCode: http.StatusNotFound, Message: "invoice PDF link not available"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, inv.Data.PDFLink, http.NoBody)
	if err != nil {
		return nil, "", err
	}
	pdfClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := pdfClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, "", &ServiceError{StatusCode: http.StatusBadGateway, Message: "billing service returned non-2xx for invoice PDF"}
	}
	return resp, invoiceID, nil
}

func (s *licensedStrategy) GetSubscription(ctx context.Context, orgID string) (*Response[BillingSubscription], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.LicenseBillingGetSubscription(ctx, lk)
}

func (s *licensedStrategy) GetSubscriptions(ctx context.Context, orgID string) (*Response[[]BillingSubscription], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.LicenseBillingGetSubscription(ctx, lk)
	if err != nil {
		return nil, err
	}
	out := []BillingSubscription{}
	if resp.Data.ID != "" {
		out = append(out, resp.Data)
	}
	return &Response[[]BillingSubscription]{Status: true, Message: resp.Message, Data: out}, nil
}

func (s *licensedStrategy) GetPaymentMethods(ctx context.Context, orgID string) (*Response[[]PaymentMethod], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.LicenseBillingGetPaymentMethods(ctx, lk)
}

func (s *licensedStrategy) GetSetupIntent(ctx context.Context, orgID string) (*Response[SetupIntent], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.LicenseBillingGetSetupIntent(ctx, lk)
}

func (s *licensedStrategy) GetPlans(ctx context.Context, orgID string) (*Response[[]Plan], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.LicenseBillingGetPlans(ctx, lk)
}

func (s *licensedStrategy) GetTaxIDTypes(ctx context.Context, orgID string) (*Response[[]TaxIDType], error) {
	lk := s.instanceLicenseKey
	if lk == "" && orgID != "" {
		k, err := s.licenseKeyFor(ctx, orgID)
		if err != nil {
			return nil, err
		}
		lk = k
	}
	if lk == "" {
		return s.client.GetTaxIDTypes(ctx)
	}
	return s.client.LicenseBillingGetTaxIDTypes(ctx, lk)
}

func (s *licensedStrategy) GetOrganisation(ctx context.Context, orgID string) (*Response[BillingOrganisation], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.LicenseBillingGetOrganisation(ctx, lk)
	if err != nil {
		return nil, err
	}
	if resp.Data.ExternalID == "" {
		resp.Data.ExternalID = orgID
	}
	if resp.Data.Name == "" && s.orgRepo != nil {
		if org, oerr := s.orgRepo.FetchOrganisationByID(ctx, orgID); oerr == nil && org != nil {
			resp.Data.Name = org.Name
		}
	}
	return resp, nil
}

func (s *licensedStrategy) GetInternalOrganisationID(ctx context.Context, orgID string) (string, error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return "", err
	}
	ctxResp, err := s.client.LicenseBillingGetContext(ctx, lk)
	if err != nil {
		return "", err
	}
	return ctxResp.Data.OrganisationID, nil
}

func (s *licensedStrategy) LicenseSummary(ctx context.Context, orgID string) LicenseSummary {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil || lk == "" {
		return LicenseSummary{}
	}
	out := LicenseSummary{MaskedKey: MaskLicenseKey(lk)}
	if s.client == nil {
		return out
	}
	ctxResp, err := s.client.LicenseBillingGetContext(ctx, lk)
	if err != nil || ctxResp == nil {
		return out
	}
	out.Configured = true
	out.HasEntitlements = ctxResp.Data.HasSubscription
	return out
}

func (s *licensedStrategy) CreateOrganisation(ctx context.Context, data BillingOrganisation) (*Response[BillingOrganisation], error) {
	return nil, ErrNoLicense
}

func (s *licensedStrategy) OnboardSubscription(ctx context.Context, orgID string, req OnboardSubscriptionRequest) (*Response[Checkout], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.SelfHostedStartCheckout(ctx, lk, SelfHostedStartCheckoutRequest(req))
}

func (s *licensedStrategy) UpgradeSubscription(ctx context.Context, orgID, subscriptionID string, req UpgradeSubscriptionRequest) (*Response[Checkout], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.LicenseBillingUpgradeSubscription(ctx, lk, req)
}

func (s *licensedStrategy) DeleteSubscription(ctx context.Context, orgID, subscriptionID string) (*Response[interface{}], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.LicenseBillingDeleteSubscription(ctx, lk)
}

func (s *licensedStrategy) SetDefaultPaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.LicenseBillingSetDefaultPaymentMethod(ctx, lk, pmID)
}

func (s *licensedStrategy) DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.LicenseBillingDeletePaymentMethod(ctx, lk, pmID)
}

func (s *licensedStrategy) UpdateOrganisation(ctx context.Context, orgID string, data BillingOrganisation) (*Response[BillingOrganisation], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.LicenseBillingUpdateOrganisation(ctx, lk, data)
}

func (s *licensedStrategy) UpdateOrganisationTaxID(ctx context.Context, orgID string, data UpdateOrganisationTaxIDRequest) (*Response[BillingOrganisation], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.LicenseBillingUpdateOrganisationTaxID(ctx, lk, data)
}

func (s *licensedStrategy) UpdateOrganisationAddress(ctx context.Context, orgID string, data UpdateOrganisationAddressRequest) (*Response[BillingOrganisation], error) {
	lk, err := s.licenseKeyFor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.client.LicenseBillingUpdateOrganisationAddress(ctx, lk, data)
}
