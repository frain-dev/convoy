package handlers

import (
	"errors"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/license"
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
//
//	@Summary		List all subscriptions
//	@Description	This endpoint fetches all the subscriptions
//	@Tags			Subscriptions
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string							true	"Project ID"
//	@Param			request		query		models.QueryListSubscription	false	"Query Params"
//	@Success		200			{object}	util.ServerResponse{data=pagedResponse{content=[]models.SubscriptionResponse}}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions [get]
func (h *Handler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var q *models.QueryListSubscription

	data := q.Transform(r)
	subscriptions, paginationData, err := postgres.NewSubscriptionRepo(h.A.DB, h.A.Cache).LoadSubscriptionsPaged(r.Context(), project.UID, data.FilterBy, data.Pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("an error occurred while fetching subscriptions")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching subscriptions", http.StatusInternalServerError))
		return
	}

	if subscriptions == nil {
		subscriptions = make([]datastore.Subscription, 0)
	}

	var org *datastore.Organisation
	orgRepo := postgres.NewOrgRepo(h.A.DB, h.A.Cache)
	org, err = orgRepo.FetchOrganisationByID(r.Context(), project.OrganisationID)
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

	baseUrl, err := h.retrieveHost()
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
//
//	@Summary		Retrieve a subscription
//	@Description	This endpoint retrieves a single subscription
//	@Tags			Subscriptions
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string	true	"Project ID"
//	@Param			subscriptionID	path		string	true	"subscription id"
//	@Success		200				{object}	util.ServerResponse{data=models.SubscriptionResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/{subscriptionID} [get]
func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subscription, err := postgres.NewSubscriptionRepo(h.A.DB, h.A.Cache).FindSubscriptionByID(r.Context(), project.UID, chi.URLParam(r, "subscriptionID"))
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
//
//	@Summary		Create a subscription
//	@Description	This endpoint creates a subscriptions
//	@Tags			Subscriptions
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string						true	"Project ID"
//	@Param			subscription	body		models.CreateSubscription	true	"Subscription details"
//	@Success		200				{object}	util.ServerResponse{data=models.SubscriptionResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions [post]
func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
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
		SubRepo:         postgres.NewSubscriptionRepo(h.A.DB, h.A.Cache),
		EndpointRepo:    postgres.NewEndpointRepo(h.A.DB, h.A.Cache),
		SourceRepo:      postgres.NewSourceRepo(h.A.DB, h.A.Cache),
		Project:         project,
		NewSubscription: &sub,
	}

	subscription, err := cs.Run(r.Context())
	if err != nil {
		h.A.Logger.WithError(err).Error("failed to create subscription")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := models.SubscriptionResponse{Subscription: subscription}
	_ = render.Render(w, r, util.NewServerResponse("Subscription created successfully", resp, http.StatusCreated))
}

// DeleteSubscription
//
//	@Summary		Delete subscription
//	@Description	This endpoint deletes a subscription
//	@Tags			Subscriptions
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string	true	"Project ID"
//	@Param			subscriptionID	path		string	true	"subscription id"
//	@Success		200				{object}	util.ServerResponse{data=Stub}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/{subscriptionID} [delete]
func (h *Handler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	sub, err := postgres.NewSubscriptionRepo(h.A.DB, h.A.Cache).FindSubscriptionByID(r.Context(), project.UID, chi.URLParam(r, "subscriptionID"))
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to find subscription")
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("failed to find subscription", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = postgres.NewSubscriptionRepo(h.A.DB, h.A.Cache).DeleteSubscription(r.Context(), project.UID, sub)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to delete subscription")
		_ = render.Render(w, r, util.NewErrorResponse("failed to delete subscription", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscription deleted successfully", nil, http.StatusOK))
}

// UpdateSubscription
//
//	@Summary		Update a subscription
//	@Description	This endpoint updates a subscription
//	@Tags			Subscriptions
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string						true	"Project ID"
//	@Param			subscriptionID	path		string						true	"subscription id"
//	@Param			subscription	body		models.UpdateSubscription	true	"Subscription Details"
//	@Success		200				{object}	util.ServerResponse{data=models.SubscriptionResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/{subscriptionID} [put]
func (h *Handler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var update models.UpdateSubscription
	err = util.ReadJSON(r, &update)
	if err != nil {
		h.A.Logger.WithError(err).Error(err.Error())
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	err = update.Validate()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	us := services.UpdateSubscriptionService{
		SubRepo:        postgres.NewSubscriptionRepo(h.A.DB, h.A.Cache),
		EndpointRepo:   postgres.NewEndpointRepo(h.A.DB, h.A.Cache),
		SourceRepo:     postgres.NewSourceRepo(h.A.DB, h.A.Cache),
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

func (h *Handler) ToggleSubscriptionStatus(w http.ResponseWriter, r *http.Request) {
	// For backward compatibility
	_ = render.Render(w, r, util.NewServerResponse("Subscription status updated successfully", nil, http.StatusAccepted))
}

// TestSubscriptionFilter
//
//	@Summary	Validate subscription filter
//	@Description	This endpoint validates that a filter will match a certain payload structure.
//	@Tags			Subscriptions
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string				true	"Project ID"
//	@Param			filter		body		models.TestFilter	true	"Filter Details"
//	@Success		200			{object}	util.ServerResponse{data=boolean}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/test_filter [post]
func (h *Handler) TestSubscriptionFilter(w http.ResponseWriter, r *http.Request) {
	valid := license.LICENSE.Load()
	if !valid {
		_ = render.Render(w, r, util.NewServiceErrResponse(convoy.ErrFeatureNotAccessible("subscription filters")))
		return
	}

	var test models.TestFilter
	err := util.ReadJSON(r, &test)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subRepo := postgres.NewSubscriptionRepo(h.A.DB, h.A.Cache)
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
//
//	@Summary		Validate subscription filter
//	@Description	This endpoint validates that a filter will match a certain payload structure.
//	@Tags			Subscriptions
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string						true	"Project ID"
//	@Param			filter		body		models.TestWebhookFunction	true	"Function Details"
//	@Success		200			{object}	util.ServerResponse{data=models.SubscriptionFunctionResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/test_function [post]
func (h *Handler) TestSubscriptionFunction(w http.ResponseWriter, r *http.Request) {
	valid := license.LICENSE.Load()
	if !valid {
		_ = render.Render(w, r, util.NewServiceErrResponse(convoy.ErrFeatureNotAccessible("subscription functions")))
		return
	}

	var test models.TestWebhookFunction
	err := util.ReadJSON(r, &test)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subRepo := postgres.NewSubscriptionRepo(h.A.DB, h.A.Cache)
	mutatedPayload, consoleLog, err := subRepo.TransformPayload(r.Context(), test.Function, test.Payload)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to transform payload")
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	functionResponse := models.SubscriptionFunctionResponse{
		Payload: mutatedPayload,
		Log:     consoleLog,
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscription transformer function run successfully", functionResponse, http.StatusOK))
}
