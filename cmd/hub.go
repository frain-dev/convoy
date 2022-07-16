package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/server"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/util"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	deviceRepo       datastore.DeviceRepository
	subscriptionRepo datastore.SubscriptionRepository
	sourceRepo       datastore.SourceRepository
	appRepo          datastore.ApplicationRepository

	lock sync.RWMutex // prevent data race on deviceClients

	// Registered clients.
	deviceClients map[string]*Client

	// Register new clients.
	register chan *Client

	// Unregister clients.
	unregister chan *Client

	events chan *CLIEvent
	close  chan struct{}
}

func NewHub(deviceRepo datastore.DeviceRepository, subscriptionRepo datastore.SubscriptionRepository, sourceRepo datastore.SourceRepository, appRepo datastore.ApplicationRepository) *Hub {
	return &Hub{
		appRepo:          appRepo,
		deviceRepo:       deviceRepo,
		subscriptionRepo: subscriptionRepo,
		sourceRepo:       sourceRepo,
		deviceClients:    map[string]*Client{},
		register:         make(chan *Client, 1),
		unregister:       make(chan *Client, 1),
		events:           make(chan *CLIEvent, 10),
		close:            make(chan struct{}),
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

			err := client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				log.WithError(err).Error("failed to set write deadline")
			}

			err = client.conn.WriteMessage(websocket.TextMessage, ev.Data)
			if err != nil {
				log.WithError(err).Error("failed to write pong message")
			}
		case <-h.close:
			return
		}
	}
}

func (h *Hub) StartEventWatcher() {
	lookupStage1 := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: m.SubscriptionCollection},
			{Key: "localField", Value: "subscription_id"},
			{Key: "foreignField", Value: "uid"},
			{Key: "as", Value: "subscription"},
		}},
	}

	matchStage := bson.D{
		{Key: "$match",
			Value: bson.D{
				{Key: "subscription.type", Value: datastore.SubscriptionTypeCLI},
			},
		},
	}

	lookupStage2 := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: m.EventCollection},
			{Key: "localField", Value: "event_id"},
			{Key: "foreignField", Value: "uid"},
			{Key: "as", Value: "event"},
		}},
	}

	addFieldStage := bson.D{
		{Key: "$addFields",
			Value: bson.D{
				{Key: "event_type", Value: "$event.event_type"},
			},
		},
	}

	unsetStage1 := bson.D{
		{Key: "$unset", Value: "event"},
	}

	unsetStage2 := bson.D{
		{Key: "$unset", Value: "subscription"},
	}

	pipeline := mongo.Pipeline{lookupStage1, matchStage, lookupStage2, addFieldStage, unsetStage1, unsetStage2}

	fn := h.watchEventDeliveriesCollection()
	err := watchCollection(fn, pipeline, m.EventCollection, h.close)
	if err != nil {
		log.WithError(err).Error("database collection watcher exited unexpectedly")
	}
}

type CLIEvent struct {
	Data      json.RawMessage
	EventType string
	DeviceID  string
	AppID     string
	GroupID   string
}

func (h *Hub) watchEventDeliveriesCollection() WatcherFn {
	return func(doc convoy.GenericMap) error {
		metadata, ok := doc["metadata"].(*datastore.Metadata)
		if !ok {
			return fmt.Errorf("event delivery metadata has wrong type of: %T", doc["metadata"])
		}

		appID, ok := doc["app_id"].(string)
		if !ok {
			return fmt.Errorf("event delivery app id has wrong type of: %T", doc["app_id"])
		}

		groupID, ok := doc["group_id"].(string)
		if !ok {
			return fmt.Errorf("event delivery group id has wrong type of: %T", doc["group_id"])
		}

		eventType, ok := doc["event_type"].(string)
		if !ok {
			return fmt.Errorf("event delivery event_type has wrong type of: %T", doc["event_type"])
		}

		deviceID, ok := doc["device_id"].(string)
		if !ok {
			return fmt.Errorf("event delivery device id has wrong type of: %T", doc["device_id"])
		}

		h.events <- &CLIEvent{
			Data:      metadata.Data,
			EventType: eventType,
			AppID:     appID,
			DeviceID:  deviceID,
			GroupID:   groupID,
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

func (h *Hub) requireApp() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser := server.GetAuthUserFromContext(r.Context())
			if len(authUser.Role.Apps) > 0 {
				appID := authUser.Role.Apps[0]
				app, err := h.appRepo.FindApplicationByID(r.Context(), appID)
				if err != nil {
					respond(w, http.StatusBadRequest, "failed to find application")
					return
				}

				r = r.WithContext(server.SetApplicationInContext(r.Context(), app))
			}

			next.ServeHTTP(w, r)
		})
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

func (h *Hub) Login(w http.ResponseWriter, r *http.Request) {
	loginRequest := &LoginRequest{}
	err := util.ReadJSON(r, &loginRequest)
	if err != nil {
		respond(w, http.StatusBadRequest, "device id is required in request body")
		return
	}

	group := server.GetGroupFromContext(r.Context())
	app, ok := getApplicationFromContext(r.Context())

	appID := ""
	if ok {
		appID = app.UID
	}

	var device *datastore.Device
	if !util.IsStringEmpty(loginRequest.DeviceID) {
		device, err = h.deviceRepo.FetchDeviceByID(r.Context(), loginRequest.DeviceID, appID, group.UID)
		if err != nil {
			respond(w, http.StatusBadRequest, "device not found")
			return
		}

		if device.GroupID != group.UID {
			respond(w, http.StatusUnauthorized, "unauthorized to access device")
			return
		}

		if device.AppID != appID {
			respond(w, http.StatusUnauthorized, "unauthorized to access device")
			return
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

		ctx, cancel := getCtx()
		defer cancel()

		err = h.deviceRepo.CreateDevice(ctx, device)
		if err != nil {
			respond(w, http.StatusBadRequest, "failed to create new device")
			return
		}
	}

	respondWithData(w, http.StatusOK, device)
}

func (h *Hub) Listen(w http.ResponseWriter, r *http.Request) {
	listenRequest := &ListenRequest{}
	err := util.ReadJSON(r, &listenRequest)
	if err != nil {
		respond(w, http.StatusBadRequest, "empty request body")
		return
	}

	group := server.GetGroupFromContext(r.Context())
	app, ok := getApplicationFromContext(r.Context())

	appID := ""
	if ok {
		appID = app.UID
	}

	ctx, cancel := getCtx()
	defer cancel()

	device, err := h.deviceRepo.FetchDeviceByID(ctx, listenRequest.DeviceID, appID, group.UID)
	if err != nil {
		respond(w, http.StatusBadRequest, "device not found")
		return
	}

	if device.GroupID != group.UID {
		respond(w, http.StatusUnauthorized, "unauthorized to access device")
		return
	}

	if device.AppID != appID {
		respond(w, http.StatusUnauthorized, "unauthorized to access device")
		return
	}

	if !util.IsStringEmpty(listenRequest.SourceID) {
		source, err := h.sourceRepo.FindSourceByID(ctx, "", listenRequest.SourceID)
		if err != nil {
			log.WithError(err).Error("error retrieving source")
			respond(w, http.StatusBadRequest, "failed to find source")
			return
		}

		if source.GroupID != group.UID {
			respond(w, http.StatusUnauthorized, "unauthorized to access source")
			return
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
			respond(w, http.StatusBadRequest, "failed to create new subscription")
			return
		}
	default:
		respond(w, http.StatusBadRequest, "failed to find subscription by id")
		return
	}

	conn, err := ug.Upgrade(w, r, nil)
	if err != nil {
		log.WithError(err).Error("failed to u pgrade connection to websocket connection")
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

func respond(w http.ResponseWriter, code int, msg string) {
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

	w.WriteHeader(code)
	_, err = w.Write(data)
	if err != nil {
		log.WithError(err).Error("failed to write response data")
	}
}
