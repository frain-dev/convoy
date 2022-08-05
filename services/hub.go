package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	// "go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	deviceRepo        datastore.DeviceRepository
	subscriptionRepo  datastore.SubscriptionRepository
	sourceRepo        datastore.SourceRepository
	appRepo           datastore.ApplicationRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	// groupRepo        datastore.GroupRepository

	lock sync.RWMutex // prevent data race on deviceClients

	// Registered clients.
	deviceClients map[string]*Client

	// Register new clients.
	register chan *Client

	// Unregister clients.
	unregister chan *Client

	watchCollectionFn WatchCollectionFn

	events chan *CLIEvent
	close  chan struct{}
}

type WatchCollectionFn func(fn func(doc map[string]interface{}) error, pipeline mongo.Pipeline, collection string, stop chan struct{}) error

var ug = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func NewHub(deviceRepo datastore.DeviceRepository, subscriptionRepo datastore.SubscriptionRepository, sourceRepo datastore.SourceRepository, appRepo datastore.ApplicationRepository, eventDeliveryRepo datastore.EventDeliveryRepository, watchCollectionFn WatchCollectionFn) *Hub {
	return &Hub{
		watchCollectionFn: watchCollectionFn,
		appRepo:           appRepo,
		deviceRepo:        deviceRepo,
		subscriptionRepo:  subscriptionRepo,
		sourceRepo:        sourceRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		deviceClients:     map[string]*Client{},
		register:          make(chan *Client, 1),
		unregister:        make(chan *Client, 1),
		events:            make(chan *CLIEvent, 10),
		close:             make(chan struct{}),
	}
}

func (h *Hub) StartEventSender() {
	for {
		select {
		case ev := <-h.events:
			h.lock.RLock()
			client := h.deviceClients[ev.DeviceID]
			h.lock.RUnlock()

			if !client.IsOnline() {
				client.GoOffline()
				continue
			}

			if client.Device.GroupID != ev.GroupID {
				continue
			}

			if !util.IsStringEmpty(client.Device.AppID) {
				if client.Device.AppID != ev.AppID {
					continue
				}
			}

			if !client.HasEventType(ev.EventType) {
				continue
			}

			j, err := json.Marshal(ev)
			if err != nil {
				log.WithError(err).Error("failed to marshal cli event")
				continue
			}

			fmt.Printf("\n[Payload] %+v \n\n", string(j))
			err = client.conn.WriteMessage(websocket.BinaryMessage, j)
			if err != nil {
				log.WithError(err).Error("failed to write event to socket")
			}
		case <-h.close:
			return
		}
	}
}

func (h *Hub) StartEventWatcher() {
	// matchStage := bson.D{
	// 	{Key: "$match",
	// 		Value: bson.D{
	// 			{Key: "cli_metadata", Value: bson.M{"$ne": nil}},
	// 		},
	// 	},
	// }

	// pipeline := mongo.Pipeline{matchStage}

	fn := h.watchEventDeliveriesCollection()
	err := h.watchCollectionFn(fn, mongo.Pipeline{}, m.EventDeliveryCollection, h.close)
	if err != nil {
		log.WithError(err).Error("database collection watcher exited unexpectedly")
	}
}

type CLIEvent struct {
	UID              string                `json:"uid"`
	ForwardedHeaders httpheader.HTTPHeader `json:"forwarded_headers" bson:"forwarded_headers"`
	Data             json.RawMessage       `json:"data"`

	// for filtering this event delivery
	EventType string `json:"-"`
	DeviceID  string `json:"-"`
	AppID     string `json:"-"`
	GroupID   string `json:"-"`
}

func (h *Hub) watchEventDeliveriesCollection() func(doc map[string]interface{}) error {
	return func(doc map[string]interface{}) error {
		var ed *datastore.EventDelivery
		b, err := json.Marshal(doc)
		if err != nil {
			return err
		}

		err = json.Unmarshal(b, &ed)
		if err != nil {
			return err
		}

		h.events <- &CLIEvent{
			UID:              ed.UID,
			Data:             ed.Metadata.Data,
			ForwardedHeaders: ed.Headers,
			EventType:        ed.CLIMetadata.EventType,
			AppID:            ed.AppID,
			DeviceID:         ed.DeviceID,
			GroupID:          ed.GroupID,
		}

		return nil
	}
}

func (h *Hub) StartRegister() {
	for {
		select {
		case client := <-h.register:
			h.lock.Lock()
			h.deviceClients[client.deviceID] = client
			h.lock.Unlock()
		case <-h.close:
			return
		}
	}
}

func (h *Hub) StartUnregister() {
	for {
		select {
		case client := <-h.unregister:
			h.lock.Lock()
			delete(h.deviceClients, client.deviceID)
			h.lock.Unlock()
		case <-h.close:
			return
		}
	}
}

func (h *Hub) Stop() {
	close(h.close)
}

func (h *Hub) UpdateEventDeliveryStatus(id string) {
	ed, err := h.eventDeliveryRepo.FindEventDeliveryByID(context.Background(), id)
	if err != nil {
		log.WithError(err).WithField("event_delivery_id", id).Error("failed to find event delivery")
	}

	err = h.eventDeliveryRepo.UpdateStatusOfEventDelivery(context.Background(), *ed, datastore.SuccessEventStatus)
	if err != nil {
		log.WithError(err).WithField("event_delivery_id", id).Error("failed to update event delivery status")
	}
}

type ListenRequest struct {
	HostName   string   `json:"host_name"`
	DeviceID   string   `json:"device_id"`
	SourceID   string   `json:"source_id"`
	EventTypes []string `json:"event_types"`
}

type LoginRequest struct {
	HostName string `json:"host_name"`
	DeviceID string `json:"device_id"`
}

type LoginResponse struct {
	Device *datastore.Device      `json:"device"`
	Group  *datastore.Group       `json:"group"`
	App    *datastore.Application `json:"app"`
}

type AckEventDelivery struct {
	UID string `json:"uid"`
}

func (h *Hub) LoginHandler(w http.ResponseWriter, r *http.Request) {
	loginRequest := &LoginRequest{}
	err := util.ReadJSON(r, &loginRequest)
	if err != nil {
		respond(w, http.StatusBadRequest, "device id is required in request body")
		return
	}

	group := middleware.GetGroupFromContext(r.Context())
	app := middleware.GetApplicationFromContext(r.Context())

	device, err := h.login(r.Context(), group, app, loginRequest)
	if err != nil {
		respond(w, err.(*util.ServiceError).ErrCode(), err.Error())
		return
	}

	lr := &LoginResponse{Device: device, Group: group, App: app}

	respondWithData(w, http.StatusOK, lr)
}

func (h *Hub) login(ctx context.Context, group *datastore.Group, app *datastore.Application, loginRequest *LoginRequest) (*datastore.Device, error) {
	appID := ""
	if app != nil {
		appID = app.UID
	}

	var device *datastore.Device
	var err error
	if !util.IsStringEmpty(loginRequest.DeviceID) {
		device, err = h.deviceRepo.FetchDeviceByID(ctx, loginRequest.DeviceID, appID, group.UID)
		if err != nil {
			log.WithError(err).Error("failed to find device by id")
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find device by id"))
		}

		if device.GroupID != group.UID {
			return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("unauthorized to access device"))
		}

		if device.AppID != appID {
			return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("unauthorized to access device"))
		}

		if device.Status != datastore.DeviceStatusOnline {
			device.Status = datastore.DeviceStatusOnline
			err = h.deviceRepo.UpdateDevice(ctx, device, device.AppID, device.GroupID)
			if err != nil {
				return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update device to online"))
			}
		}
	} else {
		device = &datastore.Device{
			UID:            uuid.NewString(),
			GroupID:        group.UID,
			AppID:          appID,
			HostName:       loginRequest.HostName,
			Status:         datastore.DeviceStatusOnline,
			DocumentStatus: datastore.ActiveDocumentStatus,
			LastSeenAt:     primitive.NewDateTimeFromTime(time.Now()),
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		}

		err = h.deviceRepo.CreateDevice(ctx, device)
		if err != nil {
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create new device"))
		}
	}

	return device, nil
}

func (h *Hub) ListenHandler(w http.ResponseWriter, r *http.Request) {
	listenRequest := &ListenRequest{}
	err := json.Unmarshal([]byte(r.Header.Get("Body")), &listenRequest)
	if err != nil {
		respond(w, http.StatusBadRequest, "empty request body")
		return
	}

	group := middleware.GetGroupFromContext(r.Context())
	app := middleware.GetApplicationFromContext(r.Context())

	device, err := h.listen(r.Context(), group, app, listenRequest)
	if err != nil {
		respond(w, err.(*util.ServiceError).ErrCode(), err.Error())
		return
	}

	conn, err := ug.Upgrade(w, r, nil)
	if err != nil {
		log.WithError(err).Error("failed to upgrade connection to websocket connection")
		respond(w, http.StatusBadRequest, "failed to upgrade connection to websocket connection")
		return
	}

	client := &Client{
		hub:        h,
		conn:       conn,
		deviceID:   device.UID,
		Device:     device,
		EventTypes: listenRequest.EventTypes,
	}

	if len(client.EventTypes) == 0 {
		client.EventTypes = []string{"*"}
	}

	client.hub.register <- client
	go client.readPump()
}

func (h *Hub) listen(ctx context.Context, group *datastore.Group, app *datastore.Application, listenRequest *ListenRequest) (*datastore.Device, error) {
	appID := ""
	if app != nil {
		appID = app.UID
	}

	device, err := h.deviceRepo.FetchDeviceByID(ctx, listenRequest.DeviceID, appID, group.UID)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("device not found"))
	}

	if device.GroupID != group.UID {
		return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("unauthorized to access device"))
	}

	if device.AppID != appID {
		return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("unauthorized to access device"))
	}

	if !util.IsStringEmpty(listenRequest.SourceID) {
		source, err := h.sourceRepo.FindSourceByID(ctx, device.GroupID, listenRequest.SourceID)
		if err != nil {
			log.WithError(err).Error("error retrieving source")
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find source"))
		}

		if source.GroupID != group.UID {
			return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("unauthorized to access source"))
		}
	}

	_, err = h.subscriptionRepo.FindSubscriptionByDeviceID(ctx, group.UID, device.UID, listenRequest.SourceID)
	switch err {
	case nil:
		break
	case datastore.ErrSubscriptionNotFound:
		s := &datastore.Subscription{
			UID:            uuid.NewString(),
			Name:           fmt.Sprintf("device-%s-subscription", device.UID),
			Type:           datastore.SubscriptionTypeCLI,
			AppID:          appID,
			GroupID:        group.UID,
			SourceID:       listenRequest.SourceID,
			DeviceID:       device.UID,
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			Status:         datastore.ActiveSubscriptionStatus,
			DocumentStatus: datastore.ActiveDocumentStatus,
		}

		err := h.subscriptionRepo.CreateSubscription(ctx, group.UID, s)
		if err != nil {
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create new subscription"))
		}
	default:
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find subscription by id"))
	}

	return device, nil
}

func respond(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err := w.Write([]byte(msg))
	if err != nil {
		log.WithError(err).Error("failed to write response message")
	}
}

func respondWithData(w http.ResponseWriter, code int, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.WithError(err).Error("failed to marshal data")
		respond(w, http.StatusInternalServerError, "failed to marshal response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err = w.Write(data)
	if err != nil {
		log.WithError(err).Error("failed to write response data")
	}
}
