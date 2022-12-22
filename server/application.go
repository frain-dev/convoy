package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func (a *ApplicationHandler) CreateApp(w http.ResponseWriter, r *http.Request) {
	newApp := struct {
		Name            string `json:"name" valid:"required~please provide your appName"`
		SupportEmail    string `json:"support_email"`
		IsDisabled      bool   `json:"is_disabled"`
		SlackWebhookURl string `json:"slack_webhook_url"`
	}{}

	err := util.ReadJSON(r, &newApp)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := util.Validate(newApp); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project := m.GetProjectFromContext(r.Context())
	uid := uuid.New().String()
	endpoint := &datastore.Endpoint{
		UID:             uid,
		ProjectID:       project.UID,
		Title:           newApp.Name,
		SupportEmail:    newApp.SupportEmail,
		SlackWebhookURL: newApp.SlackWebhookURl,
		Status:          datastore.ActiveEndpointStatus,
		AppID:           uid,
		CreatedAt:       primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:       primitive.NewDateTimeFromTime(time.Now()),
	}

	if newApp.IsDisabled {
		endpoint.Status = datastore.InactiveEndpointStatus
	}

	endpointRepo := mongo.NewEndpointRepo(a.A.Store)
	err = endpointRepo.CreateEndpoint(r.Context(), endpoint, project.UID)
	if err != nil {
		msg := "failed to create application"
		if err == datastore.ErrDuplicateEndpointName {
			msg = fmt.Sprintf("%v: %s", datastore.ErrDuplicateEndpointName, endpoint.Title)
		}
		log.WithError(err).Error(msg)
		_ = render.Render(w, r, util.NewErrorResponse(msg, http.StatusBadRequest))
		return
	}

	app := generateAppResponse(endpoint)
	_ = render.Render(w, r, util.NewServerResponse("App created successfully", app, http.StatusCreated))
}

func (a *ApplicationHandler) GetApps(w http.ResponseWriter, r *http.Request) {
	project := m.GetProjectFromContext(r.Context())
	endpointRepo := mongo.NewEndpointRepo(a.A.Store)
	q := r.URL.Query().Get("q")
	pageable := m.GetPageableFromContext(r.Context())

	endpoints, paginationData, err := endpointRepo.LoadEndpointsPaged(r.Context(), project.UID, q, pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to load apps")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching apps. Error: "+err.Error(), http.StatusBadRequest))
		return
	}

	var appsResponse []datastore.Application
	appResponseMap := make(map[string]*datastore.Application, 0)

	for _, endpoint := range endpoints {
		ap, ok := appResponseMap[endpoint.AppID]
		endpointResp := generateEndpointResponse(endpoint)

		if ok {
			ap.Endpoints = append(ap.Endpoints, endpointResp)
		} else {
			ap := generateAppResponse(&endpoint)

			if !util.IsStringEmpty(endpoint.TargetURL) {
				ap.Endpoints = []datastore.DeprecatedEndpoint{endpointResp}
			}

			appResponseMap[endpoint.AppID] = ap
		}
	}

	for _, app := range appResponseMap {
		appsResponse = append(appsResponse, *app)
	}

	_ = render.Render(w, r, util.NewServerResponse("Apps fetched successfully",
		pagedResponse{Content: &appsResponse, Pagination: &paginationData}, http.StatusOK))
}

func (a *ApplicationHandler) GetApp(w http.ResponseWriter, r *http.Request) {
	endpoints := m.GetEndpointsFromContext(r.Context())
	app := generateAppResponse(&endpoints[0])

	for _, endpoint := range endpoints {
		endpointResp := generateEndpointResponse(endpoint)
		app.Endpoints = append(app.Endpoints, endpointResp)
	}

	_ = render.Render(w, r, util.NewServerResponse("App fetched successfully", app, http.StatusOK))
}

func (a *ApplicationHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	endpoints := m.GetEndpointsFromContext(r.Context())
	project := m.GetProjectFromContext(r.Context())
	endpointRepo := mongo.NewEndpointRepo(a.A.Store)

	appUpdate := struct {
		Name            *string `json:"name" valid:"required~please provide your appName"`
		SupportEmail    *string `json:"support_email" valid:"email~please provide a valid email"`
		IsDisabled      *bool   `json:"is_disabled"`
		SlackWebhookURL *string `json:"slack_webhook_url"`
	}{}

	err := util.ReadJSON(r, &appUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := util.Validate(appUpdate); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	for _, endpoint := range endpoints {
		endpoint.Title = *appUpdate.Name

		if appUpdate.IsDisabled != nil && endpoint.Status != datastore.PendingEndpointStatus {
			if *appUpdate.IsDisabled {
				endpoint.Status = datastore.InactiveEndpointStatus
			} else {
				endpoint.Status = datastore.ActiveEndpointStatus
			}
		}

		if appUpdate.SlackWebhookURL != nil {
			endpoint.SlackWebhookURL = *appUpdate.SlackWebhookURL
		}

		if appUpdate.SupportEmail != nil {
			endpoint.SupportEmail = *appUpdate.SupportEmail
		}

		err := endpointRepo.UpdateEndpoint(r.Context(), &endpoint, project.UID)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}
	}

	e := endpoints[0]
	app := generateAppResponse(&e)

	_ = render.Render(w, r, util.NewServerResponse("App updated successfully", app, http.StatusAccepted))
}

func (a *ApplicationHandler) DeleteApp(w http.ResponseWriter, r *http.Request) {
	endpointRepo := mongo.NewEndpointRepo(a.A.Store)

	endpoints := m.GetEndpointsFromContext(r.Context())
	for _, endpoint := range endpoints {
		err := endpointRepo.DeleteEndpoint(r.Context(), &endpoint)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}
	}

	_ = render.Render(w, r, util.NewServerResponse("App deleted successfully", nil, http.StatusOK))
}

func (a *ApplicationHandler) CreateAppEndpoint(w http.ResponseWriter, r *http.Request) {
	project := m.GetProjectFromContext(r.Context())

	endpoints := m.GetEndpointsFromContext(r.Context())
	es := createEndpointService(a)

	req := struct {
		URL                string   `json:"url"`
		Description        string   `json:"description"`
		Events             []string `json:"events"`
		AdvancedSignatures bool     `json:"advanced_signatures"`
		Secret             string   `json:"secret"`

		HttpTimeout       string                            `json:"http_timeout"`
		RateLimit         int                               `json:"rate_limit"`
		RateLimitDuration string                            `json:"rate_limit_duration"`
		Authentication    *datastore.EndpointAuthentication `json:"authentication"`
	}{}

	err := util.ReadJSON(r, &req)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := util.Validate(req); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var endpoint *datastore.Endpoint

	appDetails := endpoints[0]
	// At this stage, this is an existing app with an existing or
	// multiple endpoints. We can go ahead to create a new endpoint
	// combining the app details along with the details passed
	// in the request.
	if len(endpoints) > 1 || (len(endpoints) == 1 && !util.IsStringEmpty(endpoints[0].TargetURL)) {
		e := models.Endpoint{
			URL:                req.URL,
			Secret:             req.Secret,
			Description:        req.Description,
			AdvancedSignatures: req.AdvancedSignatures,
			Name:               appDetails.Title,
			SupportEmail:       appDetails.SupportEmail,
			SlackWebhookURL:    appDetails.SlackWebhookURL,
			HttpTimeout:        req.HttpTimeout,
			RateLimit:          req.RateLimit,
			RateLimitDuration:  req.RateLimitDuration,
			Authentication:     req.Authentication,
			AppID:              appDetails.AppID,
		}

		endpoint, err = es.CreateEndpoint(r.Context(), e, project.UID)
		if err != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(err))
			return
		}
	} else {
		// This is a new app that was just created, so we should update the existing
		// resource with the request details.
		endpoint = &appDetails
		url, err := util.CleanEndpoint(req.URL)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}

		req.URL = url
		if req.RateLimit == 0 {
			req.RateLimit = convoy.RATE_LIMIT
		}

		if util.IsStringEmpty(req.RateLimitDuration) {
			req.RateLimitDuration = convoy.RATE_LIMIT_DURATION
		}

		duration, err := time.ParseDuration(req.RateLimitDuration)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}

		if util.IsStringEmpty(req.Secret) {
			sc, err := util.GenerateSecret()
			if err != nil {
				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
				return
			}

			endpoint.Secrets = []datastore.Secret{
				{
					UID:       uuid.NewString(),
					Value:     sc,
					CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
				},
			}
		} else {
			endpoint.Secrets = append(endpoint.Secrets, datastore.Secret{
				UID:       uuid.NewString(),
				Value:     req.Secret,
				CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
			})
		}

		endpoint.Description = req.Description
		endpoint.TargetURL = req.URL
		endpoint.RateLimit = req.RateLimit
		endpoint.RateLimitDuration = duration.String()
		endpoint.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())
		auth, err := services.ValidateEndpointAuthentication(req.Authentication)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}

		endpoint.Authentication = auth

		endpointRepo := mongo.NewEndpointRepo(a.A.Store)
		err = endpointRepo.UpdateEndpoint(r.Context(), endpoint, project.UID)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}

	}

	endpointResponse := generateEndpointResponse(*endpoint)
	_ = render.Render(w, r, util.NewServerResponse("App endpoint created successfully", endpointResponse, http.StatusCreated))
}

func (a *ApplicationHandler) GetAppEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints := m.GetEndpointsFromContext(r.Context())

	endpointResponse := make([]datastore.DeprecatedEndpoint, 0)

	if util.IsStringEmpty(endpoints[0].TargetURL) {
		_ = render.Render(w, r, util.NewServerResponse("App endpoints fetched successfully", endpointResponse, http.StatusOK))
		return
	}

	for _, endpoint := range endpoints {
		resp := generateEndpointResponse(endpoint)
		endpointResponse = append(endpointResponse, resp)
	}

	_ = render.Render(w, r, util.NewServerResponse("App endpoints fetched successfully", endpointResponse, http.StatusOK))
}

func (a *ApplicationHandler) GetAppEndpoint(w http.ResponseWriter, r *http.Request) {
	endpoint := m.GetEndpointFromContext(r.Context())
	resp := generateEndpointResponse(*endpoint)

	_ = render.Render(w, r, util.NewServerResponse("App endpoint fetched successfully", resp, http.StatusOK))
}

func (a *ApplicationHandler) UpdateAppEndpoint(w http.ResponseWriter, r *http.Request) {
	endpoint := m.GetEndpointFromContext(r.Context())
	endpointService := createEndpointService(a)

	req := struct {
		URL                string   `json:"url"`
		Description        string   `json:"description"`
		Events             []string `json:"events"`
		AdvancedSignatures *bool    `json:"advanced_signatures"`
		Secret             string   `json:"secret"`

		HttpTimeout       string                            `json:"http_timeout"`
		RateLimit         int                               `json:"rate_limit"`
		RateLimitDuration string                            `json:"rate_limit_duration"`
		Authentication    *datastore.EndpointAuthentication `json:"authentication"`
	}{}

	err := util.ReadJSON(r, &req)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := util.Validate(req); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	e := models.UpdateEndpoint{
		URL:                req.URL,
		Secret:             req.Secret,
		Description:        req.Description,
		AdvancedSignatures: req.AdvancedSignatures,
		Name:               &endpoint.Title,
		SupportEmail:       &endpoint.SupportEmail,
		SlackWebhookURL:    &endpoint.SlackWebhookURL,
		HttpTimeout:        req.HttpTimeout,
		RateLimit:          req.RateLimit,
		RateLimitDuration:  req.RateLimitDuration,
		Authentication:     req.Authentication,
	}

	endpoint, err = endpointService.UpdateEndpoint(r.Context(), e, endpoint)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpointResponse := generateEndpointResponse(*endpoint)
	_ = render.Render(w, r, util.NewServerResponse("App endpoint updated successfully", endpointResponse, http.StatusAccepted))
}

func (a *ApplicationHandler) DeleteAppEndpoint(w http.ResponseWriter, r *http.Request) {
	endpoint := m.GetEndpointFromContext(r.Context())
	es := createEndpointService(a)

	err := es.DeleteEndpoint(r.Context(), endpoint)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App endpoint deleted successfully", nil, http.StatusOK))
}

func generateAppResponse(endpoint *datastore.Endpoint) *datastore.Application {
	return &datastore.Application{
		UID:             endpoint.AppID,
		ProjectID:       endpoint.ProjectID,
		Title:           endpoint.Title,
		SupportEmail:    endpoint.SupportEmail,
		SlackWebhookURL: endpoint.SlackWebhookURL,
		CreatedAt:       endpoint.CreatedAt,
		UpdatedAt:       endpoint.UpdatedAt,
	}
}

func generateEndpointResponse(endpoint datastore.Endpoint) datastore.DeprecatedEndpoint {
	return datastore.DeprecatedEndpoint{
		UID:                endpoint.UID,
		TargetURL:          endpoint.TargetURL,
		Description:        endpoint.Description,
		Secrets:            endpoint.Secrets,
		AdvancedSignatures: endpoint.AdvancedSignatures,
		HttpTimeout:        endpoint.HttpTimeout,
		RateLimit:          endpoint.RateLimit,
		RateLimitDuration:  endpoint.RateLimitDuration,
		Authentication:     endpoint.Authentication,
		CreatedAt:          endpoint.CreatedAt,
		UpdatedAt:          endpoint.UpdatedAt,
	}
}
