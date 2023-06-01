package portalapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/searcher"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func (a *PortalLinkHandler) ReplayEndpointEvent(w http.ResponseWriter, r *http.Request) {
	event, err := a.retrieveEvent(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	rs := services.ReplayEventService{
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB),
		Queue:        a.A.Queue,
		Event:        event,
	}

	err = rs.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint event replayed successfully", event, http.StatusOK))
}

func (a *PortalLinkHandler) BatchReplayEvents(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	f := &datastore.Filter{
		Project: project,
		Pageable: datastore.Pageable{
			Direction:  datastore.Next,
			PerPage:    1000000000000, // large number so we get everything in most cases
			NextCursor: datastore.DefaultCursor,
		},
		SourceID:     r.URL.Query().Get("sourceId"),
		EndpointID:   r.URL.Query().Get("endpointId"),
		SearchParams: searchParams,
	}

	bs := services.BatchReplayEventService{
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB),
		Queue:        a.A.Queue,
		EventRepo:    postgres.NewEventRepo(a.A.DB),
		Filter:       f,
	}

	successes, failures, err := bs.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("%d successful, %d failed", successes, failures), nil, http.StatusOK))
}

func (a *PortalLinkHandler) CountAffectedEvents(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	f := &datastore.Filter{
		Project: project,
		Pageable: datastore.Pageable{
			Direction:  datastore.Next,
			PerPage:    1000000000000, // large number so we get everything in most cases
			NextCursor: datastore.DefaultCursor,
		},
		SourceID:     r.URL.Query().Get("sourceId"),
		EndpointID:   r.URL.Query().Get("endpointId"),
		SearchParams: searchParams,
	}

	count, err := postgres.NewEventRepo(a.A.DB).CountEvents(r.Context(), project.UID, f)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("an error occurred while fetching event")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching event", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("events count successful", map[string]interface{}{"num": count}, http.StatusOK))
}

func (a *PortalLinkHandler) GetEndpointEvent(w http.ResponseWriter, r *http.Request) {
	event, err := a.retrieveEvent(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint event fetched successfully", event, http.StatusOK))
}

func (a *PortalLinkHandler) GetEventDelivery(w http.ResponseWriter, r *http.Request) {
	eventDelivery, err := a.retrieveEventDelivery(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Event Delivery fetched successfully", eventDelivery, http.StatusOK))
}

func (a *PortalLinkHandler) ResendEventDelivery(w http.ResponseWriter, r *http.Request) {
	eventDelivery, err := a.retrieveEventDelivery(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	fr := services.RetryEventDeliveryService{
		EventDeliveryRepo: postgres.NewEventDeliveryRepo(a.A.DB),
		EndpointRepo:      postgres.NewEndpointRepo(a.A.DB),
		Queue:             a.A.Queue,
		EventDelivery:     eventDelivery,
		Project:           project,
	}

	err = fr.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App event processed for retry successfully",
		eventDelivery, http.StatusOK))
}

func (a *PortalLinkHandler) BatchRetryEventDelivery(w http.ResponseWriter, r *http.Request) {
	status := make([]datastore.EventDeliveryStatus, 0)

	for _, s := range r.URL.Query()["status"] {
		if !util.IsStringEmpty(s) {
			status = append(status, datastore.EventDeliveryStatus(s))
		}
	}

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	portalLink, err := a.retrievePortalLink(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointIDs := getEndpointIDs(r)
	pLinkEndpoints := portalLink.Endpoints

	valid := fetchPortalLinkEndpoints(endpointIDs, pLinkEndpoints)
	if !valid {
		_ = render.Render(w, r, util.NewServiceErrResponse(errors.New("unauthorized")))
		return
	}

	f := &datastore.Filter{
		Project:     project,
		EndpointIDs: endpointIDs,
		EventID:     r.URL.Query().Get("eventId"),
		Status:      status,
		Pageable: datastore.Pageable{
			Direction:  datastore.Next,
			PerPage:    1000000000000, // large number so we get everything in most cases
			NextCursor: datastore.DefaultCursor,
		},
		SearchParams: searchParams,
	}

	br := services.BatchRetryEventDeliveryService{
		EventDeliveryRepo: postgres.NewEventDeliveryRepo(a.A.DB),
		EndpointRepo:      postgres.NewEndpointRepo(a.A.DB),
		Queue:             a.A.Queue,
		EventRepo:         postgres.NewEventRepo(a.A.DB),
		Filter:            f,
	}

	successes, failures, err := br.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("%d successful, %d failed", successes, failures), nil, http.StatusOK))
}

func fetchPortalLinkEndpoints(endpointIDs []string, pLinkEndpoints []string) bool {
	for _, id := range endpointIDs {
		for _, plinkId := range pLinkEndpoints {
			if id == plinkId {
				continue
			}

			return false
		}
	}

	return true
}

func (a *PortalLinkHandler) CountAffectedEventDeliveries(w http.ResponseWriter, r *http.Request) {
	status := make([]datastore.EventDeliveryStatus, 0)
	for _, s := range r.URL.Query()["status"] {
		if !util.IsStringEmpty(s) {
			status = append(status, datastore.EventDeliveryStatus(s))
		}
	}

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	portalLink, err := a.retrievePortalLink(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointIDs := getEndpointIDs(r)
	pLinkEndpoints := portalLink.Endpoints

	valid := fetchPortalLinkEndpoints(endpointIDs, pLinkEndpoints)
	if !valid {
		_ = render.Render(w, r, util.NewServiceErrResponse(errors.New("unauthorized")))
		return
	}

	f := &datastore.Filter{
		Project:      project,
		EndpointIDs:  endpointIDs,
		EventID:      r.URL.Query().Get("eventId"),
		Status:       status,
		SearchParams: searchParams,
	}

	count, err := postgres.NewEventDeliveryRepo(a.A.DB).CountEventDeliveries(r.Context(), project.UID, f.EndpointIDs, f.EventID, f.Status, f.SearchParams)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("an error occurred while fetching event deliveries")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching event deliveries", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("event deliveries count successful", map[string]interface{}{"num": count}, http.StatusOK))
}

func (a *PortalLinkHandler) ForceResendEventDeliveries(w http.ResponseWriter, r *http.Request) {
	eventDeliveryIDs := models.IDs{}

	err := json.NewDecoder(r.Body).Decode(&eventDeliveryIDs)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	fr := services.ForceResendEventDeliveriesService{
		EventDeliveryRepo: postgres.NewEventDeliveryRepo(a.A.DB),
		EndpointRepo:      postgres.NewEndpointRepo(a.A.DB),
		Queue:             a.A.Queue,
		IDs:               eventDeliveryIDs.IDs,
		Project:           project,
	}

	successes, failures, err := fr.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("%d successful, %d failed", successes, failures), nil, http.StatusOK))
}

func (a *PortalLinkHandler) GetEventsPaged(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Get()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	pageable := m.GetPageableFromContext(r.Context())
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	portalLink, err := a.retrievePortalLink(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	query := r.URL.Query().Get("query")
	endpointIDs := portalLink.Endpoints
	sourceID := getSourceIDs(r)[0]

	f := &datastore.Filter{
		Query:        query,
		Project:      project,
		EndpointIDs:  endpointIDs,
		SourceID:     sourceID,
		Pageable:     pageable,
		SearchParams: searchParams,
	}

	if cfg.Search.Type == config.TypesenseSearchProvider && !util.IsStringEmpty(query) {
		searchBackend, err := searcher.NewSearchClient(cfg)
		if err != nil {
			log.FromContext(r.Context()).WithError(err).Error("failed to initialise search backend")
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}

		se := services.SearchEventService{
			EventRepo: postgres.NewEventRepo(a.A.DB),
			Searcher:  searchBackend,
			Filter:    f,
		}

		m, paginationData, err := se.Run(r.Context())
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}
		_ = render.Render(w, r, util.NewServerResponse("Endpoint events fetched successfully",
			pagedResponse{Content: &m, Pagination: &paginationData}, http.StatusOK))

		return
	}

	m, paginationData, err := postgres.NewEventRepo(a.A.DB).LoadEventsPaged(r.Context(), project.UID, f)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to fetch events")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching app events", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App events fetched successfully",
		pagedResponse{Content: &m, Pagination: &paginationData}, http.StatusOK))
}

func (a *PortalLinkHandler) GetEventDeliveriesPaged(w http.ResponseWriter, r *http.Request) {
	status := make([]datastore.EventDeliveryStatus, 0)
	for _, s := range r.URL.Query()["status"] {
		if !util.IsStringEmpty(s) {
			status = append(status, datastore.EventDeliveryStatus(s))
		}
	}

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	portalLink, err := a.retrievePortalLink(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointIDs := portalLink.Endpoints

	f := &datastore.Filter{
		Project:      project,
		EventID:      r.URL.Query().Get("eventId"),
		EndpointIDs:  endpointIDs,
		Status:       status,
		Pageable:     m.GetPageableFromContext(r.Context()),
		SearchParams: searchParams,
	}

	ed, paginationData, err := postgres.NewEventDeliveryRepo(a.A.DB).LoadEventDeliveriesPaged(r.Context(), project.UID, f.EndpointIDs, f.EventID, f.Status, f.SearchParams, f.Pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to fetch event deliveries")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching event deliveries", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Event deliveries fetched successfully",
		pagedResponse{Content: &ed, Pagination: &paginationData}, http.StatusOK))
}

func getSearchParams(r *http.Request) (datastore.SearchParams, error) {
	var searchParams datastore.SearchParams
	format := "2006-01-02T15:04:05"
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")

	var err error

	var startT time.Time
	if len(startDate) == 0 {
		startT = time.Unix(0, 0)
	} else {
		startT, err = time.Parse(format, startDate)
		if err != nil {
			return searchParams, errors.New("please specify a startDate in the format " + format)
		}
	}
	var endT time.Time
	if len(endDate) == 0 {
		now := time.Now()
		endT = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
	} else {
		endT, err = time.Parse(format, endDate)
		if err != nil {
			return searchParams, errors.New("please specify a correct endDate in the format " + format + " or none at all")
		}
	}

	if err := m.EnsurePeriod(startT, endT); err != nil {
		return searchParams, err
	}

	searchParams = datastore.SearchParams{
		CreatedAtStart: startT.Unix(),
		CreatedAtEnd:   endT.Unix(),
	}

	return searchParams, nil
}

func (a *PortalLinkHandler) retrieveEvent(r *http.Request) (*datastore.Event, error) {
	project, err := a.retrieveProject(r)
	if err != nil {
		return &datastore.Event{}, err
	}

	eventID := chi.URLParam(r, "eventID")
	eventRepo := postgres.NewEventRepo(a.A.DB)
	return eventRepo.FindEventByID(r.Context(), project.UID, eventID)
}

func (a *PortalLinkHandler) retrieveEventDelivery(r *http.Request) (*datastore.EventDelivery, error) {
	project, err := a.retrieveProject(r)
	if err != nil {
		return &datastore.EventDelivery{}, err
	}

	eventDeliveryID := chi.URLParam(r, "eventDeliveryID")
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.A.DB)
	return eventDeliveryRepo.FindEventDeliveryByID(r.Context(), project.UID, eventDeliveryID)
}

func getEndpointIDs(r *http.Request) []string {
	var endpoints []string

	for _, id := range r.URL.Query()["endpointId"] {
		if !util.IsStringEmpty(id) {
			endpoints = append(endpoints, id)
		}
	}

	return endpoints
}

func getSourceIDs(r *http.Request) []string {
	var sourceIDs []string

	for _, id := range r.URL.Query()["sourceId"] {
		if !util.IsStringEmpty(id) {
			sourceIDs = append(sourceIDs, id)
		}
	}

	return sourceIDs
}
