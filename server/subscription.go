package server

import (
	"net/http"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/services"

	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createSubscriptionService(a *ApplicationHandler) *services.SubcriptionService {
	subRepo := postgres.NewSubscriptionRepo(a.A.DB)
	endpointRepo := postgres.NewEndpointRepo(a.A.DB)
	sourceRepo := postgres.NewSourceRepo(a.A.DB)

	return services.NewSubscriptionService(subRepo, endpointRepo, sourceRepo)
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
	var endpoints []string

	pageable := m.GetPageableFromContext(r.Context())
	project := m.GetProjectFromContext(r.Context())
	endpointID := m.GetEndpointIDFromContext(r)
	endpointIDs := m.GetEndpointIDsFromContext(r.Context())

	if !util.IsStringEmpty(endpointID) {
		endpoints = []string{endpointID}
	}

	if len(endpointIDs) > 0 {
		endpoints = endpointIDs
	}

	filter := &datastore.FilterBy{ProjectID: project.UID, EndpointIDs: endpoints}

	subService := createSubscriptionService(a)
	subscriptions, paginationData, err := subService.LoadSubscriptionsPaged(r.Context(), filter, pageable)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to load subscriptions")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	org := m.GetOrganisationFromContext(r.Context())
	var customDomain string
	if org == nil {
		customDomain = ""
	} else {
		customDomain = org.CustomDomain.ValueOrZero()
	}

	baseUrl := m.GetHostFromContext(r.Context())
	for i := range subscriptions {
		fillSourceURL(subscriptions[i].Source, baseUrl, customDomain)
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
// @Param subscriptionID path string true "subscription id"
// @Success 200 {object} util.ServerResponse{data=datastore.Subscription}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/subscriptions/{subscriptionID} [get]
func (a *ApplicationHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	subId := chi.URLParam(r, "subscriptionID")
	project := m.GetProjectFromContext(r.Context())

	subService := createSubscriptionService(a)
	subscription, err := subService.FindSubscriptionByID(r.Context(), project, subId, false)
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
	project := m.GetProjectFromContext(r.Context())

	var sub models.Subscription
	err := util.ReadJSON(r, &sub)
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
	project := m.GetProjectFromContext(r.Context())
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
		a.A.Logger.WithError(err).Error(err.Error())
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	g := m.GetProjectFromContext(r.Context())
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
// Deprecated
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
	// For backward compatibility
	_ = render.Render(w, r, util.NewServerResponse("Subscription status updated successfully", nil, http.StatusAccepted))
}

// TestSubscriptionFilter
// @Summary Test subscription filter
// @Description This endpoint tests a subscription's filter
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param projectID path string true "Project id"
// @Param filter body models.TestFilter true "Filter Details"
// @Success 200 {object} util.ServerResponse{data=boolean}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/subscriptions/test_filter [post]
func (a *ApplicationHandler) TestSubscriptionFilter(w http.ResponseWriter, r *http.Request) {
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
