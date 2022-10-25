package server

import (
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/services"

	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createSubscriptionService(a *ApplicationHandler) *services.SubcriptionService {
	subRepo := mongo.NewSubscriptionRepo(a.A.Store)
	appRepo := mongo.NewApplicationRepo(a.A.Store)
	sourceRepo := mongo.NewSourceRepo(a.A.Store)

	return services.NewSubscriptionService(subRepo, appRepo, sourceRepo)
}

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
// @Param projectID path string true "Project id"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]datastore.Subscription}}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/subscriptions [get]
func (a *ApplicationHandler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	group := m.GetGroupFromContext(r.Context())
	appID := m.GetAppIDFromContext(r)

	filter := &datastore.FilterBy{GroupID: group.UID, AppID: appID}

	subService := createSubscriptionService(a)
	subscriptions, paginationData, err := subService.LoadSubscriptionsPaged(r.Context(), filter, pageable)
	if err != nil {
		log.WithError(err).Error("failed to load subscriptions")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscriptions fetched successfully",
		pagedResponse{Content: &subscriptions, Pagination: &paginationData}, http.StatusOK))
}

// GetSubscription
// @Summary Gets a subscription
// @Description This endpoint fetches an Subscription by it's id
// @Tags Subscriptions
// @Accept json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param subscriptionID path string true "application id"
// @Success 200 {object} util.ServerResponse{data=datastore.Subscription}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/subscriptions/{subscriptionID} [get]
func (a *ApplicationHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	subId := chi.URLParam(r, "subscriptionID")
	group := m.GetGroupFromContext(r.Context())

	subService := createSubscriptionService(a)
	subscription, err := subService.FindSubscriptionByID(r.Context(), group, subId, false)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscription fetched successfully", subscription, http.StatusOK))
}

// CreateSubscription
// @Summary Creates a subscription
// @Description This endpoint creates a subscriptions
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param projectID path string true "Project id"
// @Param subscription body models.Subscription true "Subscription details"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]datastore.Subscription}}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/subscriptions [post]
func (a *ApplicationHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	group := m.GetGroupFromContext(r.Context())

	var sub models.Subscription
	err := util.ReadJSON(r, &sub)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subService := createSubscriptionService(a)
	subscription, err := subService.CreateSubscription(r.Context(), group, &sub)
	if err != nil {
		log.WithError(err).Error("failed to create subscription")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscriptions created successfully", subscription, http.StatusCreated))
}

// DeleteSubscription
// @Summary Delete subscription
// @Description This endpoint deletes a subscription
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param projectID path string true "Project id"
// @Param subscriptionID path string true "subscription id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/subscriptions/{subscriptionID} [delete]
func (a *ApplicationHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	group := m.GetGroupFromContext(r.Context())
	subService := createSubscriptionService(a)

	sub, err := subService.FindSubscriptionByID(r.Context(), group, chi.URLParam(r, "subscriptionID"), true)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = subService.DeleteSubscription(r.Context(), group.UID, sub)
	if err != nil {
		log.Errorln("failed to delete subscription - ", err)
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
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
// @Param projectID path string true "Project id"
// @Param subscriptionID path string true "subscription id"
// @Param subscription body models.Subscription true "Subscription Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Subscription}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/subscriptions/{subscriptionID} [put]
func (a *ApplicationHandler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	var update models.UpdateSubscription
	err := util.ReadJSON(r, &update)
	if err != nil {
		log.WithError(err).Error(err.Error())
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	g := m.GetGroupFromContext(r.Context())
	subscription := chi.URLParam(r, "subscriptionID")

	subService := createSubscriptionService(a)
	sub, err := subService.UpdateSubscription(r.Context(), g.UID, subscription, &update)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscription updated successfully", sub, http.StatusAccepted))
}

// ToggleSubscriptionStatus
// @Summary Toggles a subscription's status from active <-> inactive
// @Description This endpoint updates a subscription
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param projectID path string true "Project id"
// @Param subscriptionID path string true "subscription id"
// @Success 200 {object} util.ServerResponse{data=datastore.Subscription}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/subscriptions/{subscriptionID}/toggle_status [put]
func (a *ApplicationHandler) ToggleSubscriptionStatus(w http.ResponseWriter, r *http.Request) {
	g := m.GetGroupFromContext(r.Context())
	subscription := chi.URLParam(r, "subscriptionID")

	subService := createSubscriptionService(a)
	sub, err := subService.ToggleSubscriptionStatus(r.Context(), g.UID, subscription)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscription status updated successfully", sub, http.StatusAccepted))
}
