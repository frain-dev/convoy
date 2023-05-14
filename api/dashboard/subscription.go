package dashboard

import (
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

func createSubscriptionService(a *DashboardHandler) *services.SubcriptionService {
	subRepo := postgres.NewSubscriptionRepo(a.A.DB)
	endpointRepo := postgres.NewEndpointRepo(a.A.DB)
	sourceRepo := postgres.NewSourceRepo(a.A.DB)

	return services.NewSubscriptionService(subRepo, endpointRepo, sourceRepo)
}

func (a *DashboardHandler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointIDs := getEndpointIDs(r)
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

func (a *DashboardHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
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

func (a *DashboardHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "project.manage", project); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
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

func (a *DashboardHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "project.manage", project); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	subService := createSubscriptionService(a)
	sub, err := subService.FindSubscriptionByID(r.Context(), project, chi.URLParam(r, "subscriptionID"), true)
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

func (a *DashboardHandler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
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

	if err = a.A.Authz.Authorize(r.Context(), "project.manage", project); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
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

func (a *DashboardHandler) TestSubscriptionFilter(w http.ResponseWriter, r *http.Request) {
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
