package dashboard

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/internal/pkg/middleware"
)

type DashboardHandler struct {
	A *types.APIOptions
}

func NewDashboardHandler(a *types.APIOptions) *DashboardHandler {
	return &DashboardHandler{A: a}
}

func (a *DashboardHandler) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {
	format := "2006-01-02T15:04:05"
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")
	if len(startDate) == 0 {
		_ = render.Render(w, r, util.NewErrorResponse("please specify a startDate query", http.StatusBadRequest))
		return
	}

	startT, err := time.Parse(format, startDate)
	if err != nil {
		a.A.Logger.WithError(err).Error("error parsing startDate")
		_ = render.Render(w, r, util.NewErrorResponse("please specify a startDate in the format "+format, http.StatusBadRequest))
		return
	}

	period := r.URL.Query().Get("type")
	if util.IsStringEmpty(period) {
		_ = render.Render(w, r, util.NewErrorResponse("please specify a type query", http.StatusBadRequest))
		return
	}

	if !datastore.IsValidPeriod(period) {
		_ = render.Render(w, r, util.NewErrorResponse("please specify a type query in (daily, weekly, monthly, yearly)", http.StatusBadRequest))
		return
	}

	var endT time.Time
	if len(endDate) == 0 {
		endT = time.Date(startT.Year(), startT.Month(), startT.Day(), 23, 59, 59, 999999999, startT.Location())
	} else {
		endT, err = time.Parse(format, endDate)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("please specify an endDate in the format "+format+" or none at all", http.StatusBadRequest))
			return
		}
	}

	p := datastore.PeriodValues[period]
	if err := middleware.EnsurePeriod(startT, endT); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("invalid period '%s': %s", period, err.Error()), http.StatusBadRequest))
		return
	}

	searchParams := datastore.SearchParams{
		CreatedAtStart: startT.Unix(),
		CreatedAtEnd:   endT.Unix(),
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	qs := fmt.Sprintf("%v:%v:%v:%v", project.UID, searchParams.CreatedAtStart, searchParams.CreatedAtEnd, period)

	var data *models.DashboardSummary
	err = a.A.Cache.Get(r.Context(), qs, &data)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to get dashboard summary from cache")
	}

	if data != nil {
		_ = render.Render(w, r, util.NewServerResponse("Dashboard summary fetched successfully",
			data, http.StatusOK))
		return
	}

	apps, err := postgres.NewEndpointRepo(a.A.DB).CountProjectEndpoints(r.Context(), project.UID)
	if err != nil {
		log.WithError(err).Error("failed to count project endpoints")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while searching apps", http.StatusInternalServerError))
		return
	}

	eventsSent, messages, err := a.computeDashboardMessages(r.Context(), project.UID, searchParams, p)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching messages", http.StatusInternalServerError))
		return
	}

	dashboard := models.DashboardSummary{
		Applications: int(apps),
		EventsSent:   eventsSent,
		Period:       period,
		PeriodData:   &messages,
	}

	err = a.A.Cache.Set(r.Context(), qs, dashboard, time.Minute)

	if err != nil {
		a.A.Logger.WithError(err)
	}

	_ = render.Render(w, r, util.NewServerResponse("Dashboard summary fetched successfully",
		dashboard, http.StatusOK))
}

func (a *DashboardHandler) computeDashboardMessages(ctx context.Context, projectID string, searchParams datastore.SearchParams, period datastore.Period) (uint64, []datastore.EventInterval, error) {
	var messagesSent uint64

	eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.A.DB)
	messages, err := eventDeliveryRepo.LoadEventDeliveriesIntervals(ctx, projectID, searchParams, period, 1)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to load message intervals - ")
		return 0, nil, err
	}

	for _, m := range messages {
		messagesSent += m.Count
	}

	return messagesSent, messages, nil
}

func (a *DashboardHandler) retrieveOrganisation(r *http.Request) (*datastore.Organisation, error) {
	orgID := chi.URLParam(r, "orgID")

	if util.IsStringEmpty(orgID) {
		orgID = r.URL.Query().Get("orgID")
	}

	orgRepo := postgres.NewOrgRepo(a.A.DB)
	return orgRepo.FetchOrganisationByID(r.Context(), orgID)
}

func (a *DashboardHandler) retrieveUser(r *http.Request) (*datastore.User, error) {
	authUser := middleware.GetAuthUserFromContext(r.Context())
	user, ok := authUser.Metadata.(*datastore.User)
	if !ok {
		return &datastore.User{}, errors.New("User not found")
	}

	return user, nil
}

func (a *DashboardHandler) retrieveMembership(r *http.Request) (*datastore.OrganisationMember, error) {
	org, err := a.retrieveOrganisation(r)
	if err != nil {
		return &datastore.OrganisationMember{}, err
	}

	user, err := a.retrieveUser(r)
	if err != nil {
		return &datastore.OrganisationMember{}, err
	}

	orgMemberRepo := postgres.NewOrgMemberRepo(a.A.DB)
	return orgMemberRepo.FetchOrganisationMemberByUserID(r.Context(), user.UID, org.UID)
}

func (a *DashboardHandler) retrieveProject(r *http.Request) (*datastore.Project, error) {
	projectID := chi.URLParam(r, "projectID")

	if util.IsStringEmpty(projectID) {
		return &datastore.Project{}, errors.New("Project ID not present in request")
	}

	projectRepo := postgres.NewProjectRepo(a.A.DB)
	return projectRepo.FetchProjectByID(r.Context(), projectID)
}

func (a *DashboardHandler) retrieveHost() (string, error) {
	cfg, err := config.Get()
	if err != nil {
		return "", err
	}

	return cfg.Host, nil
}
