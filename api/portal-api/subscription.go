package portalapi

import (
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"

	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createSubscriptionService(a *PortalLinkHandler) *services.SubcriptionService {
	subRepo := postgres.NewSubscriptionRepo(a.A.DB)
	endpointRepo := postgres.NewEndpointRepo(a.A.DB)
	sourceRepo := postgres.NewSourceRepo(a.A.DB)

	return services.NewSubscriptionService(subRepo, endpointRepo, sourceRepo)
}

func (a *PortalLinkHandler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	portalLink, err := a.retrievePortalLink(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	pageable := m.GetPageableFromContext(r.Context())
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointIDs := portalLink.Endpoints
	filter := &datastore.FilterBy{ProjectID: project.UID, EndpointIDs: endpointIDs}

	subService := createSubscriptionService(a)
	subscriptions, paginationData, err := subService.LoadSubscriptionsPaged(r.Context(), filter, pageable)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to load subscriptions")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	var customDomain string
	if org == nil {
		customDomain = ""
	} else {
		customDomain = org.CustomDomain.ValueOrZero()
	}

	baseUrl, err := a.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	for i := range subscriptions {
		fillSourceURL(subscriptions[i].Source, baseUrl, customDomain)
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscriptions fetched successfully",
		pagedResponse{Content: &subscriptions, Pagination: &paginationData}, http.StatusOK))
}

func (a *PortalLinkHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	subId := chi.URLParam(r, "subscriptionID")
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	subService := createSubscriptionService(a)
	subscription, err := subService.FindSubscriptionByID(r.Context(), project, subId, false)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscription fetched successfully", subscription, http.StatusOK))
}

func (a *PortalLinkHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	var sub models.Subscription
	err = util.ReadJSON(r, &sub)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subService := createSubscriptionService(a)
	subscription, err := subService.CreateSubscription(r.Context(), project, &sub)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to create subscription")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscription created successfully", subscription, http.StatusCreated))
}

func (a *PortalLinkHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}
	subService := createSubscriptionService(a)

	sub, err := subService.FindSubscriptionByID(r.Context(), project, chi.URLParam(r, "subscriptionID"), true)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = subService.DeleteSubscription(r.Context(), project.UID, sub)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to delete subscription")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscription deleted successfully", nil, http.StatusOK))
}

func (a *PortalLinkHandler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	var update models.UpdateSubscription
	err := util.ReadJSON(r, &update)
	if err != nil {
		a.A.Logger.WithError(err).Error(err.Error())
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	subscription := chi.URLParam(r, "subscriptionID")

	subService := createSubscriptionService(a)
	sub, err := subService.UpdateSubscription(r.Context(), project.UID, subscription, &update)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscription updated successfully", sub, http.StatusAccepted))
}

func (a *PortalLinkHandler) TestSubscriptionFilter(w http.ResponseWriter, r *http.Request) {
	var test models.TestFilter
	err := util.ReadJSON(r, &test)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subService := createSubscriptionService(a)

	isBodyValid, err := subService.TestSubscriptionFilter(r.Context(), test.Request.Body, test.Schema.Body)
	if err != nil {
		a.A.Logger.WithError(err).Error("an error occured while validating the subscription filter")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	isHeaderValid, err := subService.TestSubscriptionFilter(r.Context(), test.Request.Headers, test.Schema.Headers)
	if err != nil {
		a.A.Logger.WithError(err).Error("an error occured while validating the subscription filter")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	isValid := isBodyValid && isHeaderValid

	_ = render.Render(w, r, util.NewServerResponse("Subscriptions filter validated successfully", isValid, http.StatusCreated))
}

func fillSourceURL(s *datastore.Source, baseUrl string, customDomain string) {
	url := baseUrl
	if len(customDomain) > 0 {
		url = customDomain
	}

	s.URL = fmt.Sprintf("%s/ingest/%s", url, s.MaskID)
}
