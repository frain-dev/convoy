package server

import (
	"net/http"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

type Application struct {
	Name            string `json:"name"`
	SupportEmail    string `json:"support_email"`
	IsDisabled      bool   `json:"is_disabled"`
	SlackWebhookURl string `json:"slack_webhook_url"`
}

func (a *ApplicationHandler) CreateApp(w http.ResponseWriter, r *http.Request) {
	var newApp Application

	err := util.ReadJSON(r, &newApp)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	group := m.GetGroupFromContext(r.Context())

	uid := uuid.New().String()
	endpoint := &datastore.Endpoint{
		UID:             uid,
		GroupID:         group.UID,
		Title:           newApp.Name,
		SupportEmail:    newApp.SupportEmail,
		SlackWebhookURL: newApp.SlackWebhookURl,
		IsDisabled:      newApp.IsDisabled,
		AppID:           uid,
		CreatedAt:       primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:       primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus:  datastore.ActiveDocumentStatus,
	}

	endpointRepo := mongo.NewEndpointRepo(a.A.Store)

	err = endpointRepo.CreateEndpoint(r.Context(), endpoint, group.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App created successfully", endpoint, http.StatusCreated))
}


func (a *ApplicationHandler) GetApp(w http.ResponseWriter, r *http.Request) {
}
