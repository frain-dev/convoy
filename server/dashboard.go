package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

type AuthorizedLogin struct {
	Username   string    `json:"username,omitempty"`
	Token      string    `json:"token"`
	ExpiryTime time.Time `json:"expiry_time"`
}

type ViewableConfiguration struct {
	Strategy  datastore.StrategyConfiguration  `json:"strategy"`
	Signature datastore.SignatureConfiguration `json:"signature"`
}

func (a *ApplicationHandler) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {
	format := "2006-01-02T15:04:05"
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")
	if len(startDate) == 0 {
		_ = render.Render(w, r, util.NewErrorResponse("please specify a startDate query", http.StatusBadRequest))
		return
	}

	startT, err := time.Parse(format, startDate)
	if err != nil {
		log.Errorln("error parsing startDate - ", err)
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
	if err := m.EnsurePeriod(startT, endT); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("invalid period '%s': %s", period, err.Error()), http.StatusBadRequest))
		return
	}

	searchParams := datastore.SearchParams{
		CreatedAtStart: startT.Unix(),
		CreatedAtEnd:   endT.Unix(),
	}

	group := m.GetGroupFromContext(r.Context())

	qs := fmt.Sprintf("%v:%v:%v:%v", group.UID, searchParams.CreatedAtStart, searchParams.CreatedAtEnd, period)

	var data *models.DashboardSummary

	err = a.S.Cache.Get(r.Context(), qs, &data)

	if err != nil {
		log.Error(err)
	}

	if data != nil {
		_ = render.Render(w, r, util.NewServerResponse("Dashboard summary fetched successfully",
			data, http.StatusOK))
		return
	}

	apps, err := a.S.AppService.CountGroupApplications(r.Context(), group.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while searching apps", http.StatusInternalServerError))
		return
	}

	eventsSent, messages, err := a.M.ComputeDashboardMessages(r.Context(), group.UID, searchParams, p)
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

	err = a.S.Cache.Set(r.Context(), qs, dashboard, time.Minute)

	if err != nil {
		log.Error(err)
	}

	_ = render.Render(w, r, util.NewServerResponse("Dashboard summary fetched successfully",
		dashboard, http.StatusOK))
}

func (a *ApplicationHandler) GetAuthLogin(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, util.NewServerResponse("Logged in successfully",
		m.GetAuthLoginFromContext(r.Context()), http.StatusOK))
}

func (a *ApplicationHandler) GetAllConfigDetails(w http.ResponseWriter, r *http.Request) {

	g := m.GetGroupFromContext(r.Context())

	viewableConfig := ViewableConfiguration{
		Strategy:  *g.Config.Strategy,
		Signature: *g.Config.Signature,
	}

	_ = render.Render(w, r, util.NewServerResponse("Config details fetched successfully",
		viewableConfig, http.StatusOK))
}
