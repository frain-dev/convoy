package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	endpointsvc "github.com/frain-dev/convoy/internal/endpoints"
	"github.com/frain-dev/convoy/internal/event_deliveries"
	"github.com/frain-dev/convoy/internal/events"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
)

func (h *Handler) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {
	format := "2006-01-02T15:04:05"
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")
	if len(startDate) == 0 {
		_ = render.Render(w, r, util.NewErrorResponse("please specify a startDate query", http.StatusBadRequest))
		return
	}

	startT, err := time.Parse(format, startDate)
	if err != nil {
		h.A.Logger.Error("error parsing startDate", "error", err)
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
	if err = middleware.EnsurePeriod(startT, endT); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("invalid period '%s': %s", period, err.Error()), http.StatusBadRequest))
		return
	}

	searchParams := datastore.SearchParams{
		CreatedAtStart: startT.Unix(),
		CreatedAtEnd:   endT.Unix(),
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointIDs := make([]string, 0)
	authUser := middleware.GetAuthUserFromContext(r.Context())
	if h.IsReqWithPortalLinkToken(authUser) {
		portalLink, innerErr := h.retrievePortalLinkFromToken(r)
		if innerErr != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(innerErr))
			return
		}

		eIDs, innerErr := h.getEndpoints(r, portalLink)
		if innerErr != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(innerErr))
			return
		}

		if len(eIDs) == 0 {
			intervals := make([]datastore.EventInterval, 0)
			dashboardPL := models.DashboardSummary{
				Applications: 0,
				EventsSent:   0,
				Period:       period,
				PeriodData:   &intervals,
			}

			_ = render.Render(w, r, util.NewServerResponse("Dashboard summary fetched successfully.",
				dashboardPL, http.StatusOK))
			return
		}

		endpointIDs = append(endpointIDs, eIDs...)
	}

	var endpoints int64
	if len(endpointIDs) == 0 {
		endpoints, err = endpointsvc.New(h.A.Logger, h.A.DB).CountProjectEndpoints(r.Context(), project.UID)
		if err != nil {
			h.A.Logger.Error("failed to count project endpoints", "error", err)
			_ = render.Render(w, r, util.NewErrorResponse("an error occurred while searching apps", http.StatusInternalServerError))
			return
		}
	} else {
		endpoints = int64(len(endpointIDs))
	}

	ctx, cancel := context.WithTimeout(r.Context(), events.SearchTimeout)
	defer cancel()

	eventsSent, messages, err := h.computeDashboardMessages(ctx, project.UID, searchParams, p, endpointIDs)
	if err != nil {
		if renderEventDeliveriesTimeout(w, r, err) {
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching messages", http.StatusInternalServerError))
		return
	}

	dashboard := models.DashboardSummary{
		Applications: int(endpoints),
		EventsSent:   eventsSent,
		Period:       period,
		PeriodData:   &messages,
	}

	_ = render.Render(w, r, util.NewServerResponse("Dashboard summary fetched successfully",
		dashboard, http.StatusOK))
}

func (h *Handler) computeDashboardMessages(ctx context.Context, projectID string, searchParams datastore.SearchParams, period datastore.Period, endpointIds []string) (uint64, []datastore.EventInterval, error) {
	var messagesSent uint64

	eventDeliveryRepo := event_deliveries.New(h.A.Logger, h.A.DB)
	messages, err := eventDeliveryRepo.LoadEventIntervals(ctx, projectID, searchParams, period, endpointIds)
	if err != nil {
		h.A.Logger.ErrorContext(ctx, "failed to load message intervals - ", "error", err)
		return 0, nil, err
	}

	for _, m := range messages {
		messagesSent += m.Count
	}

	return messagesSent, messages, nil
}
