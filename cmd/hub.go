package main

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/datastore/mongo"
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

	lock sync.RWMutex
	// Registered clients.
	deviceClients map[string]*Client

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	events chan *CLIEvent
	close  chan struct{}
}

func NewHub(deviceRepo datastore.DeviceRepository, subscriptionRepo datastore.SubscriptionRepository, sourceRepo datastore.SourceRepository) *Hub {
	return &Hub{
		deviceRepo:       deviceRepo,
		subscriptionRepo: subscriptionRepo,
		sourceRepo:       sourceRepo,
		deviceClients:    map[string]*Client{},
		register:         make(chan *Client),
		unregister:       make(chan *Client),
		events:           make(chan *CLIEvent),
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

			err := client.conn.WriteMessage(websocket.TextMessage, ev.Data)
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

func (h *Hub) RemoveClient(c *Client) {
	h.lock.Lock()
	delete(h.deviceClients, c.deviceID)
	h.lock.Unlock()
}

func (h *Hub) Stop() {
	close(h.close)
}
