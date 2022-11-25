package socket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/gorilla/websocket"

	"go.mongodb.org/mongo-driver/mongo"
)

// Register new clients
var register chan *Client

// Unregister clients
var unregister chan *Client

// events from the change stream are written to this channel and are sent to the respective devices
var events chan *CLIEvent

// Hub maintains the set of active clients and broadcasts messages to the clients.
type Hub struct {
	lock sync.RWMutex // prevent data race on deviceClients

	// Registered clients.
	deviceClients map[string]*Client

	watchCollectionFn WatchCollectionFn

	close chan struct{}

	// this ticker is used to periodically set inactive (or incorrectly disconnected) devices to offline
	ticker *time.Ticker
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

func NewHub() *Hub {
	register = make(chan *Client, 1)
	unregister = make(chan *Client, 1)
	events = make(chan *CLIEvent, 10)

	return &Hub{
		watchCollectionFn: watchCollection,
		deviceClients:     map[string]*Client{},
		close:             make(chan struct{}),
	}
}

func (h *Hub) StartEventSender() {
	for {
		select {
		case ev := <-events:
			h.lock.RLock()
			client := h.deviceClients[ev.DeviceID]
			h.lock.RUnlock()

			// there is no valid client for this event delivery, so skip it
			if client == nil {
				continue
			}

			if !client.IsOnline() {
				client.GoOffline()
				client.Close(unregister)
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
	err := h.watchCollectionFn(fn, datastore.EventDeliveryCollection, h.close)
	if err != nil {
		log.WithError(err).Fatal("database collection watcher exited unexpectedly")
	}
}

func (h *Hub) watchEventDeliveriesCollection() func(doc map[string]interface{}) {
	return func(doc map[string]interface{}) {
		var ed *datastore.EventDelivery
		b, err := json.Marshal(doc)
		if err != nil {
			log.WithError(err).Error("failed to marshal doc")
			return
		}

		err = json.Unmarshal(b, &ed)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal json")
			return
		}

		// this isn't a cli event deliery
		if ed.CLIMetadata == nil {
			return
		}

		// map[Data:base64Str Subtype:int]
		var dataMap convoy.GenericMap
		err = json.Unmarshal(ed.Metadata.Data, &dataMap)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal metadata")
			return
		}

		value, exists := dataMap["Data"]
		if !exists {
			log.Error("'Data' field doesn't exist in map")
			return
		}

		vBytes, err := json.Marshal(value)
		if err != nil {
			log.Error(err)
			return
		}

		vStr, err := strconv.Unquote(string(vBytes))
		if err != nil {
			log.Error(err)
			return
		}

		dataBytes, err := base64.StdEncoding.DecodeString(vStr)
		if err != nil {
			log.Error(err)
			return
		}

		events <- &CLIEvent{
			UID:       ed.UID,
			Data:      dataBytes,
			Headers:   ed.Headers,
			EventType: ed.CLIMetadata.EventType,
			AppID:     ed.AppID,
			DeviceID:  ed.DeviceID,
			GroupID:   ed.GroupID,
		}
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

func (h *Hub) StartClientStatusWatcher() {
	h.ticker = time.NewTicker(time.Second * 30)
	defer h.ticker.Stop()

	for {
		select {
		case <-h.ticker.C:
			for k, v := range h.deviceClients {
				h.lock.Lock()
				if !h.deviceClients[k].IsOnline() {
					h.deviceClients[k].GoOffline()
					h.deviceClients[k].Close(unregister)
					log.Printf("%s has be set to offline after inactivity for 30 seconds", v.Device.HostName)
				}
				h.lock.Unlock()
			}
		case <-h.close:
			return
		}
	}
}

func (h *Hub) Stop() {
	close(h.close)
}

type WatchCollectionFn func(fn func(doc map[string]interface{}), collection string, stop chan struct{}) error

func watchCollection(fn func(map[string]interface{}), collection string, stop chan struct{}) error {
	cfg, err := config.Get()
	if err != nil {
		return err
	}

	if cfg.Database.Type != "mongodb" {
		return convoy.ErrUnsupportedDatebase
	}

	client, err := m.New(cfg)
	if err != nil {
		return err
	}

	db := client.Client().(*mongo.Database)
	coll := db.Collection(collection)
	ctx := context.Background()

	cs, err := coll.Watch(ctx, mongo.Pipeline{})
	if err != nil {
		return err
	}
	defer cs.Close(ctx)

	for {
		select {
		case <-stop:
			log.Println("Exiting Database watcher")
			return nil
		default:
			ok := cs.Next(ctx)
			if ok {
				var document *convoy.GenericMap
				err := cs.Decode(&document)
				if err != nil {
					return err
				}

				if (*document)["operationType"].(string) == "insert" {
					doc := (*document)["fullDocument"].(convoy.GenericMap)
					fn(doc)
				}
			}
		}
	}
}
