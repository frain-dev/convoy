package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	mongopagination "github.com/gobeam/mongo-go-pagination"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type applicationHandler struct {
	appRepo convoy.ApplicationRepository
	orgRepo convoy.OrganisationRepository
	msgRepo convoy.MessageRepository
}

type pagedResponse struct {
	Content    interface{}                     `json:"content,omitempty"`
	Pagination *mongopagination.PaginationData `json:"pagination,omitempty"`
}

func newApplicationHandler(msgRepo convoy.MessageRepository, appRepo convoy.ApplicationRepository, orgRepo convoy.OrganisationRepository) *applicationHandler {

	return &applicationHandler{
		msgRepo: msgRepo,
		appRepo: appRepo,
		orgRepo: orgRepo,
	}
}

func (a *applicationHandler) GetApp(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App fetched successfully",
		*getApplicationFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetApps(w http.ResponseWriter, r *http.Request) {
	pageable := getPageableFromContext(r.Context())

	apps, paginationData, err := a.appRepo.LoadApplicationsPaged(r.Context(), "", pageable)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching apps", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("Apps fetched successfully",
		pagedResponse{Content: &apps, Pagination: &paginationData}, http.StatusOK))
}

func (a *applicationHandler) CreateApp(w http.ResponseWriter, r *http.Request) {

	var newApp models.Application
	err := json.NewDecoder(r.Body).Decode(&newApp)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	appName := newApp.AppName
	if util.IsStringEmpty(appName) {
		_ = render.Render(w, r, newErrorResponse("please provide your appName", http.StatusBadRequest))
		return
	}
	orgId := newApp.OrgID
	if util.IsStringEmpty(orgId) {
		_ = render.Render(w, r, newErrorResponse("please provide your orgId", http.StatusBadRequest))
		return
	}

	org, err := a.orgRepo.FetchOrganisationByID(r.Context(), orgId)
	if err != nil {
		msg := "an error occurred while fetching organisation"
		statusCode := http.StatusInternalServerError

		if errors.Is(err, convoy.ErrOrganisationNotFound) {
			msg = err.Error()
			statusCode = http.StatusBadRequest
		}
		_ = render.Render(w, r, newErrorResponse(msg, statusCode))
		return
	}

	if util.IsStringEmpty(newApp.Secret) {
		newApp.Secret, err = util.GenerateSecret()
		if err != nil {
			_ = render.Render(w, r, newErrorResponse(fmt.Sprintf("could not generate secret...%v", err.Error()), http.StatusInternalServerError))
			return
		}
	}

	uid := uuid.New().String()
	app := &convoy.Application{
		UID:            uid,
		OrgID:          org.UID,
		Title:          appName,
		Secret:         newApp.Secret,
		SupportEmail:   newApp.SupportEmail,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		Endpoints:      []convoy.Endpoint{},
		DocumentStatus: convoy.ActiveDocumentStatus,
	}

	err = a.appRepo.CreateApplication(r.Context(), app)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while creating app", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("App created successfully", app, http.StatusCreated))
}

func (a *applicationHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	var appUpdate models.Application
	err := json.NewDecoder(r.Body).Decode(&appUpdate)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	appName := appUpdate.AppName
	if util.IsStringEmpty(appName) {
		_ = render.Render(w, r, newErrorResponse("please provide your appName", http.StatusBadRequest))
		return
	}

	app := getApplicationFromContext(r.Context())

	app.Title = appName
	if !util.IsStringEmpty(appUpdate.Secret) {
		app.Secret = appUpdate.Secret
	}

	if !util.IsStringEmpty(appUpdate.SupportEmail) {
		app.SupportEmail = appUpdate.SupportEmail
	}

	err = a.appRepo.UpdateApplication(r.Context(), app)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while updating app", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("App updated successfully", app, http.StatusAccepted))
}

func (a *applicationHandler) DeleteApp(w http.ResponseWriter, r *http.Request) {
	app := getApplicationFromContext(r.Context())
	err := a.appRepo.DeleteApplication(r.Context(), app)
	if err != nil {
		log.Errorln("failed to delete app - ", err)
		_ = render.Render(w, r, newErrorResponse("an error occurred while deleting app", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("App deleted successfully", nil, http.StatusOK))
}

func (a *applicationHandler) CreateAppEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.Endpoint
	e, err := parseEndpointFromBody(r.Body)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	appID := chi.URLParam(r, "appID")
	app, err := a.appRepo.FindApplicationByID(r.Context(), appID)
	if err != nil {

		msg := "an error occurred while retrieving app details"
		statusCode := http.StatusInternalServerError

		if errors.Is(err, convoy.ErrApplicationNotFound) {
			msg = err.Error()
			statusCode = http.StatusNotFound
		}

		_ = render.Render(w, r, newErrorResponse(msg, statusCode))
		return
	}

	endpoint := &convoy.Endpoint{
		UID:            uuid.New().String(),
		TargetURL:      e.URL,
		Description:    e.Description,
		Status:         convoy.ActiveEndpointStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: convoy.ActiveDocumentStatus,
	}

	app.Endpoints = append(app.Endpoints, *endpoint)

	err = a.appRepo.UpdateApplication(r.Context(), app)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while adding app endpoint", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("App endpoint created successfully", endpoint, http.StatusCreated))
}

func (a *applicationHandler) GetAppEndpoint(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App endpoint fetched successfully",
		*getApplicationEndpointFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetAppEndpoints(w http.ResponseWriter, r *http.Request) {
	app := getApplicationFromContext(r.Context())

	app.Endpoints = filterDeletedEndpoints(app.Endpoints)
	_ = render.Render(w, r, newServerResponse("App endpoints fetched successfully", app.Endpoints, http.StatusOK))
}

func (a *applicationHandler) UpdateAppEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.Endpoint
	e, err := parseEndpointFromBody(r.Body)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	app := getApplicationFromContext(r.Context())
	endPointId := chi.URLParam(r, "endpointID")

	endpoints, endpoint, err := updateEndpointIfFound(&app.Endpoints, endPointId, e)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	app.Endpoints = *endpoints
	err = a.appRepo.UpdateApplication(r.Context(), app)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while updating app endpoints", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("Apps endpoint updated successfully", endpoint, http.StatusAccepted))
}

func (a *applicationHandler) DeleteAppEndpoint(w http.ResponseWriter, r *http.Request) {
	app := getApplicationFromContext(r.Context())
	e := getApplicationEndpointFromContext(r.Context())

	for i, endpoint := range app.Endpoints {
		if endpoint.UID == e.UID && endpoint.DeletedAt == 0 {
			app.Endpoints = append(app.Endpoints[:i], app.Endpoints[i+1:]...)
			break
		}
	}

	err := a.appRepo.UpdateApplication(r.Context(), app)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while deleting app endpoint", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("App endpoint deleted successfully", nil, http.StatusOK))
}

func (a *applicationHandler) GetPaginatedApps(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Apps fetched successfully",
		pagedResponse{Content: *getApplicationsFromContext(r.Context()),
			Pagination: getPaginationDataFromContext(r.Context())}, http.StatusOK))
}
