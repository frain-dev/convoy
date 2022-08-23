package socket

import (
	"encoding/json"
	"sync"

	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/util"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"go.mongodb.org/mongo-driver/mongo"
)

// Register new clients.
var register chan *Client

// Unregister clients.
var unregister chan *Client

// Hub maintains the set of active clients and broadcasts messages to the clients.
type Hub struct {
	lock sync.RWMutex // prevent data race on deviceClients

	// Registered clients.
	deviceClients map[string]*Client

	watchCollectionFn WatchCollectionFn

	close  chan struct{}
	events chan *CLIEvent
}

type AckEventDelivery struct {
	UID string `json:"uid"`
}

type CLIEvent struct {
	UID     string                `json:"uid"`
	Headers httpheader.HTTPHeader `json:"headers" bson:"headers"`
	Data    json.RawMessage       `json:"data"`

	// for filtering this event delivery
	EventType string `json:"-"`
	DeviceID  string `json:"-"`
	AppID     string `json:"-"`
	GroupID   string `json:"-"`
}

type WatchCollectionFn func(fn func(doc map[string]interface{}) error, pipeline mongo.Pipeline, collection string, stop chan struct{}) error

func NewHub(watchCollectionFn WatchCollectionFn) *Hub {
	register = make(chan *Client, 1)
	unregister = make(chan *Client, 1)

	return &Hub{
		watchCollectionFn: watchCollectionFn,
		deviceClients:     map[string]*Client{},
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

			// there is no valid client for this event delivery, so skip it
			if client == nil {
				continue
			}

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
	fn := h.watchEventDeliveriesCollection()
	err := h.watchCollectionFn(fn, mongo.Pipeline{}, m.EventDeliveryCollection, h.close)
	if err != nil {
		log.WithError(err).Error("database collection watcher exited unexpectedly")
	}
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

		if ed.CLIMetadata == nil {
			return nil
		}

		h.events <- &CLIEvent{
			UID:       ed.UID,
			Data:      ed.Metadata.Data,
			Headers:   ed.Headers,
			EventType: ed.CLIMetadata.EventType,
			AppID:     ed.AppID,
			DeviceID:  ed.DeviceID,
			GroupID:   ed.GroupID,
		}

		return nil
	}
}

func (h *Hub) StartRegister() {
	for {
		select {
		case client := <-register:
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
		case client := <-unregister:
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
