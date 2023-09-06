package public

import (
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"

	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// GetSubscriptions
// @Summary List all subscriptions
// @Description This endpoint fetches all the subscriptions
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param request query models.QueryListSubscription false "Query Params"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]models.SubscriptionResponse}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/subscriptions [get]
func (a *PublicHandler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryListSubscription
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	data := q.Transform(r)
	subscriptions, paginationData, err := postgres.NewSubscriptionRepo(a.A.DB).LoadSubscriptionsPaged(r.Context(), project.UID, data.FilterBy, data.Pageable)
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

// GetSubscription
// @Summary Retrieve a subscription
// @Description This endpoint retrieves a single subscription
// @Tags Subscriptions
// @Accept json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param subscriptionID path string true "subscription id"
// @Success 200 {object} util.ServerResponse{data=models.SubscriptionResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/subscriptions/{subscriptionID} [get]
func (a *PublicHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	subscription, err := postgres.NewSubscriptionRepo(a.A.DB).FindSubscriptionByID(r.Context(), project.UID, chi.URLParam(r, "subscriptionID"))
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

// CreateSubscription
// @Summary Create a subscription
// @Description This endpoint creates a subscriptions
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param subscription body models.CreateSubscription true "Subscription details"
// @Success 200 {object} util.ServerResponse{data=models.SubscriptionResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/subscriptions [post]
func (a *PublicHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
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

// DeleteSubscription
// @Summary Delete subscription
// @Description This endpoint deletes a subscription
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param subscriptionID path string true "subscription id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/subscriptions/{subscriptionID} [delete]
func (a *PublicHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
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

// UpdateSubscription
// @Summary Update a subscription
// @Description This endpoint updates a subscription
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param subscriptionID path string true "subscription id"
// @Param subscription body models.UpdateSubscription true "Subscription Details"
// @Success 200 {object} util.ServerResponse{data=models.SubscriptionResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/subscriptions/{subscriptionID} [put]
func (a *PublicHandler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
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

func (a *PublicHandler) ToggleSubscriptionStatus(w http.ResponseWriter, r *http.Request) {
	// For backward compatibility
	_ = render.Render(w, r, util.NewServerResponse("Subscription status updated successfully", nil, http.StatusAccepted))
}

// TestSubscriptionFilter
// @Summary Validate subscription filter
// @Description This endpoint validates that a filter will match a certain payload structure.
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param filter body models.TestFilter true "Filter Details"
// @Success 200 {object} util.ServerResponse{data=boolean}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/subscriptions/test_filter [post]
func (a *PublicHandler) TestSubscriptionFilter(w http.ResponseWriter, r *http.Request) {
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

	_ = render.Render(w, r, util.NewServerResponse("Subscriptions filter validated successfully", isValid, http.StatusOK))
}

// TestSubscriptionFunction
// @Summary Validate subscription filter
// @Description This endpoint validates that a filter will match a certain payload structure.
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param filter body models.TestWebhookFunction true "Function Details"
// @Success 200 {object} util.ServerResponse{data=boolean}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/subscriptions/test_function [post]
func (a *PublicHandler) TestSubscriptionFunction(w http.ResponseWriter, r *http.Request) {
	var test models.TestWebhookFunction
	err := util.ReadJSON(r, &test)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subRepo := postgres.NewSubscriptionRepo(a.A.DB)
	mutatedPayload, err := subRepo.TransformPayload(r.Context(), test.Function, test.Payload)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to transform payload")
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscriptions filter validated successfully", mutatedPayload, http.StatusOK))
}
