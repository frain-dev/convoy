package portalapi

import (
	"errors"
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

	endpointIDs, err := a.getEndpoints(r, portalLink)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if len(endpointIDs) == 0 {
		_ = render.Render(w, r, util.NewServerResponse("Subscriptions fetched successfully",
			pagedResponse{Content: endpointIDs, Pagination: &datastore.PaginationData{PerPage: int64(pageable.PerPage)}}, http.StatusOK))
		return
	}

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

	resp := models.NewListResponse(subscriptions, func(subscription datastore.Subscription) models.SubscriptionResponse {
		return models.SubscriptionResponse{Subscription: &subscription}
	})
	_ = render.Render(w, r, util.NewServerResponse("Subscriptions fetched successfully",
		pagedResponse{Content: &resp, Pagination: &paginationData}, http.StatusOK))
}

func (a *PortalLinkHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	subId := chi.URLParam(r, "subscriptionID")
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	subscription, err := postgres.NewSubscriptionRepo(a.A.DB).FindSubscriptionByID(r.Context(), project.UID, subId)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to find subscription")
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("failed to find subscription", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.SubscriptionResponse{Subscription: subscription}
	_ = render.Render(w, r, util.NewServerResponse("Subscription fetched successfully", resp, http.StatusOK))
}

func (a *PortalLinkHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	var sub models.CreateSubscription
	err = util.ReadJSON(r, &sub)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	err = sub.Validate()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	cs := services.CreateSubscriptionService{
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

	resp := models.SubscriptionResponse{Subscription: subscription}
	_ = render.Render(w, r, util.NewServerResponse("Subscription created successfully", resp, http.StatusCreated))
}

func (a *PortalLinkHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	sub, err := postgres.NewSubscriptionRepo(a.A.DB).FindSubscriptionByID(r.Context(), project.UID, chi.URLParam(r, "subscriptionID"))
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to find subscription")
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("failed to find subscription", http.StatusNotFound))
			return
		}
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

	err = update.Validate()
	if err != nil {
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

	resp := models.SubscriptionResponse{Subscription: sub}
	_ = render.Render(w, r, util.NewServerResponse("Subscription updated successfully", resp, http.StatusAccepted))
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

func (a *PortalLinkHandler) getEndpoints(r *http.Request, pl *datastore.PortalLink) ([]string, error) {
	results := make([]string, 0)
	if !util.IsStringEmpty(pl.OwnerID) {
		endpointRepo := postgres.NewEndpointRepo(a.A.DB)
		endpoints, err := endpointRepo.FindEndpointsByOwnerID(r.Context(), pl.ProjectID, pl.OwnerID)
		if err != nil {
			return nil, err
		}

		for _, endpoint := range endpoints {
			results = append(results, endpoint.UID)
		}

		return results, nil
	}

	if pl.Endpoints == nil {
		return results, nil
	}

	return pl.Endpoints, nil
}
