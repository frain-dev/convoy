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

type OwnerEmailResolver func(ctx context.Context, orgID string) (string, error)

type cloudStrategy struct {
	client       Client
	logger       logger.Logger
	orgRepo      datastore.OrganisationRepository
	host         string
	resolveOwner OwnerEmailResolver
}

func (s *cloudStrategy) Mode() config.BillingMode { return config.BillingModeCloud }

func (s *cloudStrategy) GetUsage(ctx context.Context, orgID string) (*Response[Usage], error) {
	return s.client.GetUsage(ctx, orgID)
}

func (s *cloudStrategy) GetInvoices(ctx context.Context, orgID string) (*Response[[]Invoice], error) {
	return s.client.GetInvoices(ctx, orgID)
}

func (s *cloudStrategy) GetInvoice(ctx context.Context, orgID, invoiceID string) (*Response[Invoice], error) {
	return s.client.GetInvoice(ctx, orgID, invoiceID)
}

func (s *cloudStrategy) DownloadInvoice(ctx context.Context, orgID, invoiceID string) (*http.Response, string, error) {
	resp, err := s.client.DownloadInvoice(ctx, orgID, invoiceID)
	return resp, "", err
}

func (s *cloudStrategy) GetSubscription(ctx context.Context, orgID string) (*Response[BillingSubscription], error) {
	resp, err := s.client.GetSubscription(ctx, orgID)
	if err != nil && isOrgNotFound(err) {
		if ensureErr := s.ensureBillingOrganisation(ctx, orgID); ensureErr != nil {
			return nil, ensureErr
		}
		resp, err = s.client.GetSubscription(ctx, orgID)
	}
	if err != nil {
		return nil, err
	}
	go s.updateBillingEmailIfEmpty(orgID)
	return resp, nil
}

func (s *cloudStrategy) GetSubscriptions(ctx context.Context, orgID string) (*Response[[]BillingSubscription], error) {
	return s.client.GetSubscriptions(ctx, orgID)
}

func (s *cloudStrategy) GetPaymentMethods(ctx context.Context, orgID string) (*Response[[]PaymentMethod], error) {
	return s.client.GetPaymentMethods(ctx, orgID)
}

func (s *cloudStrategy) GetSetupIntent(ctx context.Context, orgID string) (*Response[SetupIntent], error) {
	return s.client.GetSetupIntent(ctx, orgID)
}

func (s *cloudStrategy) GetPlans(ctx context.Context, orgID string) (*Response[[]Plan], error) {
	return s.client.GetPlans(ctx)
}

func (s *cloudStrategy) GetTaxIDTypes(ctx context.Context, orgID string) (*Response[[]TaxIDType], error) {
	return s.client.GetTaxIDTypes(ctx)
}

func (s *cloudStrategy) GetOrganisation(ctx context.Context, orgID string) (*Response[BillingOrganisation], error) {
	resp, err := s.client.GetOrganisation(ctx, orgID)
	if err != nil && isOrgNotFound(err) {
		if ensureErr := s.ensureBillingOrganisation(ctx, orgID); ensureErr != nil {
			return nil, ensureErr
		}
		resp, err = s.client.GetOrganisation(ctx, orgID)
	}
	if err != nil {
		return nil, err
	}
	go s.updateBillingEmailIfEmpty(orgID)
	return resp, nil
}

func (s *cloudStrategy) GetInternalOrganisationID(ctx context.Context, orgID string) (string, error) {
	resp, err := s.GetOrganisation(ctx, orgID)
	if err != nil {
		return "", err
	}
	return resp.Data.ID, nil
}

func (s *cloudStrategy) LicenseSummary(ctx context.Context, orgID string) LicenseSummary {
	return LicenseSummary{}
}

func (s *cloudStrategy) OnboardSubscription(ctx context.Context, orgID string, req OnboardSubscriptionRequest) (*Response[Checkout], error) {
	return s.client.OnboardSubscription(ctx, orgID, req)
}

func (s *cloudStrategy) UpgradeSubscription(ctx context.Context, orgID, subscriptionID string, req UpgradeSubscriptionRequest) (*Response[Checkout], error) {
	return s.client.UpgradeSubscription(ctx, orgID, subscriptionID, req)
}

func (s *cloudStrategy) DeleteSubscription(ctx context.Context, orgID, subscriptionID string) (*Response[interface{}], error) {
	return s.client.DeleteSubscription(ctx, orgID, subscriptionID)
}

func (s *cloudStrategy) SetDefaultPaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error) {
	return s.client.SetDefaultPaymentMethod(ctx, orgID, pmID)
}

func (s *cloudStrategy) DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error) {
	return s.client.DeletePaymentMethod(ctx, orgID, pmID)
}

func (s *cloudStrategy) UpdateOrganisation(ctx context.Context, orgID string, data BillingOrganisation) (*Response[BillingOrganisation], error) {
	return s.client.UpdateOrganisation(ctx, orgID, data)
}

func (s *cloudStrategy) UpdateOrganisationTaxID(ctx context.Context, orgID string, data UpdateOrganisationTaxIDRequest) (*Response[BillingOrganisation], error) {
	return s.client.UpdateOrganisationTaxID(ctx, orgID, data)
}

func (s *cloudStrategy) UpdateOrganisationAddress(ctx context.Context, orgID string, data UpdateOrganisationAddressRequest) (*Response[BillingOrganisation], error) {
	return s.client.UpdateOrganisationAddress(ctx, orgID, data)
}

func isOrgNotFound(err error) bool {
	if err == nil {
		return false
	}
	if serviceErr, ok := errors.AsType[*ServiceError](err); ok && serviceErr.StatusCode == http.StatusNotFound {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "organisation") && strings.Contains(s, "not found")
}

func (s *cloudStrategy) ensureBillingOrganisation(ctx context.Context, orgID string) error {
	if s.orgRepo == nil {
		return errors.New("organisation repository unavailable for billing recovery")
	}
	org, err := s.orgRepo.FetchOrganisationByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("fetch organisation: %w", err)
	}
	if s.host == "" {
		return errors.New("organisation host (assigned domain) is required for billing")
	}

	ownerEmail, err := s.ownerEmail(ctx, orgID)
	if err != nil {
		return fmt.Errorf("resolve owner email: %w", err)
	}
	if ownerEmail == "" {
		return errors.New("organisation owner email is required for billing")
	}

	_, err = s.client.CreateOrganisation(ctx, BillingOrganisation{
		Name:         org.Name,
		ExternalID:   orgID,
		BillingEmail: ownerEmail,
		Host:         s.host,
	})
	return err
}

func (s *cloudStrategy) ownerEmail(ctx context.Context, orgID string) (string, error) {
	if s.resolveOwner == nil {
		return "", nil
	}
	return s.resolveOwner(ctx, orgID)
}

func (s *cloudStrategy) updateBillingEmailIfEmpty(orgID string) {
	bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := s.client.GetOrganisation(bgCtx, orgID)
	if err != nil || resp.Data.BillingEmail != "" {
		return
	}
	email, _ := s.ownerEmail(bgCtx, orgID)
	if email == "" {
		return
	}
	if _, err := s.client.UpdateOrganisation(bgCtx, orgID, BillingOrganisation{BillingEmail: email}); err != nil && s.logger != nil {
		s.logger.Warnf("failed to update billing_email for organisation %s: %v", orgID, err)
	}
}
