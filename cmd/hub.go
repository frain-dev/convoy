package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

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
		deviceRepo:       deviceRepo,
		subscriptionRepo: subscriptionRepo,
		sourceRepo:       sourceRepo,
		appRepo:          appRepo,
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
