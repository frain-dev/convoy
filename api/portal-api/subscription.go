package portalapi

import (
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"

	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

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

	subscriptions, paginationData, err := postgres.NewSubscriptionRepo(a.A.DB).LoadSubscriptionsPaged(r.Context(), project.UID, filter, pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("an error occurred while fetching subscriptions")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching subscriptions", http.StatusInternalServerError))
		return
	}

	if subscriptions == nil {
		subscriptions = make([]datastore.Subscription, 0)
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

	fs := services.FindSubscriptionByIDService{
		SubRepo:        postgres.NewSubscriptionRepo(a.A.DB),
		EndpointRepo:   postgres.NewEndpointRepo(a.A.DB),
		SourceRepo:     postgres.NewSourceRepo(a.A.DB),
		Project:        project,
		SubscriptionId: subId,
		SkipCache:      false,
	}

	subscription, err := fs.Run(r.Context())
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

	cs := services.CreateSubcriptionService{
		SubRepo:         postgres.NewSubscriptionRepo(a.A.DB),
		EndpointRepo:    postgres.NewEndpointRepo(a.A.DB),
		SourceRepo:      postgres.NewSourceRepo(a.A.DB),
		Project:         project,
		NewSubscription: &sub,
	}

	subscription, err := cs.Run(r.Context())
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

	fs := services.FindSubscriptionByIDService{
		SubRepo:        postgres.NewSubscriptionRepo(a.A.DB),
		EndpointRepo:   postgres.NewEndpointRepo(a.A.DB),
		SourceRepo:     postgres.NewSourceRepo(a.A.DB),
		Project:        project,
		SubscriptionId: chi.URLParam(r, "subscriptionID"),
		SkipCache:      true,
	}

	sub, err := fs.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = postgres.NewSubscriptionRepo(a.A.DB).DeleteSubscription(r.Context(), project.UID, sub)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to delete subscription")
		_ = render.Render(w, r, util.NewErrorResponse("failed to delete subscription", http.StatusBadRequest))
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

	us := services.UpdateSubscriptionService{
		SubRepo:        postgres.NewSubscriptionRepo(a.A.DB),
		EndpointRepo:   postgres.NewEndpointRepo(a.A.DB),
		SourceRepo:     postgres.NewSourceRepo(a.A.DB),
		ProjectId:      project.UID,
		SubscriptionId: chi.URLParam(r, "subscriptionID"),
		Update:         &update,
	}

	sub, err := us.Run(r.Context())
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

	subRepo := postgres.NewSubscriptionRepo(a.A.DB)
	isBodyValid, err := subRepo.TestSubscriptionFilter(r.Context(), test.Request.Body, test.Schema.Body)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to validate subscription filter")
		_ = render.Render(w, r, util.NewErrorResponse("failed to validate subscription filter", http.StatusBadRequest))
		return
	}

	isHeaderValid, err := subRepo.TestSubscriptionFilter(r.Context(), test.Request.Headers, test.Schema.Headers)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to validate subscription filter")
		_ = render.Render(w, r, util.NewErrorResponse("failed to validate subscription filter", http.StatusBadRequest))
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
