package server

import (
	"net/http"

	"github.com/frain-dev/convoy/datastore"

	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
)

// GetSubscriptions
// @Summary Get all subscriptions
// @Description This endpoint fetches all the subscriptions
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Param q query string false "subscription title"
// @Param groupId query string true "group id"
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]datastore.Subscription}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /subscriptions [get]
func (a *applicationHandler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	pageable := getPageableFromContext(r.Context())
	group := getGroupFromContext(r.Context())

	apps, paginationData, err := a.subService.LoadSubscriptionsPaged(r.Context(), group.UID, pageable)
	if err != nil {
		log.WithError(err).Error("failed to load subscriptions")
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("Subscriptions fetched successfully",
		pagedResponse{Content: &apps, Pagination: &paginationData}, http.StatusOK))
}

// GetSubscription
// @Summary Gets a subscription
// @Description This endpoint fetches an Subscription by it's id
// @Tags Subscription
// @Accept json
// @Produce  json
// @Param groupId query string true "group id"
// @Param subscriptionID path string true "application id"
// @Success 200 {object} serverResponse{data=datastore.Subscription}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /subscriptions/{subscriptionID} [get]
func (a *applicationHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	subId := chi.URLParam(r, "subscriptionID")
	group := getGroupFromContext(r.Context())

	subscription, err := a.subService.FindSubscriptionByID(r.Context(), group.UID, subId)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	// only incoming groups have sources
	if group.Type == datastore.IncomingGroup && subscription.SourceID != "" {
		source, err := a.sourceService.FindSourceByID(r.Context(), group, subscription.SourceID)
		if err != nil {
			_ = render.Render(w, r, newServiceErrResponse(err))
			return
		}
		subscription.Source = source
	}

	if subscription.EndpointID != "" {
		endpoint, err := a.appRepo.FindApplicationEndpointByID(r.Context(), subscription.AppID, subscription.EndpointID)
		if err != nil {
			_ = render.Render(w, r, newServiceErrResponse(err))
			return
		}

		subscription.Endpoint = endpoint
	}

	if subscription.AppID != "" {
		app, err := a.appRepo.FindApplicationByID(r.Context(), subscription.AppID)
		if err != nil {
			_ = render.Render(w, r, newServiceErrResponse(err))
			return
		}

		subscription.App = app
	}

	_ = render.Render(w, r, newServerResponse("Subscription fetched successfully", subscription, http.StatusOK))
}

// CreateSubscription
// @Summary Creates a subscription
// @Description This endpoint creates a subscriptions
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param groupId query string true "group id"
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]datastore.Subscription}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /subscriptions [post]
func (a *applicationHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	group := getGroupFromContext(r.Context())

	var s models.Subscription
	err := util.ReadJSON(r, &s)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	s.Type = string(group.Type)

	subscription, err := a.subService.CreateSubscription(r.Context(), group.UID, &s)
	if err != nil {
		log.WithError(err).Error("failed to create subscription")
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("Subscriptions created successfully", subscription, http.StatusCreated))
}

// DeleteSubscription
// @Summary Delete subscription
// @Description This endpoint deletes a subscription
// @Tags Application
// @Accept json
// @Produce json
// @Param groupId query string true "group id"
// @Param subscriptionID path string true "subscription id"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /subscriptions/{subscriptionID} [delete]
func (a *applicationHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	group := getGroupFromContext(r.Context())

	sub, err := a.subService.FindSubscriptionByID(r.Context(), group.UID, chi.URLParam(r, "subscriptionID"))
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	err = a.subService.DeleteSubscription(r.Context(), group.UID, sub)
	if err != nil {
		log.Errorln("failed to delete subscription - ", err)
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("Subscription deleted successfully", nil, http.StatusOK))
}

// UpdateSubscription
// @Summary Update a subscription
// @Description This endpoint updates a subscription
// @Tags Subscription
// @Accept json
// @Produce json
// @Param subscriptionID path string true "subscription id"
// @Param subscription body models.Subscription true "Subscription Details"
// @Success 200 {object} serverResponse{data=datastore.Subscription}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /subscriptions/{subscriptionID} [put]
func (a *applicationHandler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	var update models.UpdateSubscription
	err := util.ReadJSON(r, &update)
	if err != nil {
		log.WithError(err).Error(err.Error())
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	g := getGroupFromContext(r.Context())
	subscription := chi.URLParam(r, "subscriptionID")

	sub, err := a.subService.UpdateSubscription(r.Context(), g.UID, subscription, &update)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("Subscription updated successfully", sub, http.StatusAccepted))
}
