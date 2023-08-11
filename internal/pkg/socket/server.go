package socket

import (
	"context"
	"encoding/json"
	"github.com/frain-dev/convoy/internal/pkg/apm"
	"sync"
	"time"

	"github.com/frain-dev/convoy/util"
	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/gorilla/websocket"
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
	EventType  string `json:"-"`
	DeviceID   string `json:"-"`
	EndpointID string `json:"-"`
	SourceID   string `json:"-"`
	ProjectID  string `json:"-"`
}

func NewHub() *Hub {
	register = make(chan *Client, 1)
	unregister = make(chan *Client, 1)
	events = make(chan *CLIEvent, 10)

	return &Hub{
		deviceClients: map[string]*Client{},
		close:         make(chan struct{}),
	}
}

func (h *Hub) Start(ctx context.Context) {
	go h.StartRegister()
	go h.StartUnregister()
	go h.StartEventSender(ctx)
	go h.StartClientStatusWatcher(ctx)
}

func (h *Hub) StartEventSender(ctx context.Context) {
	for {
		select {
		case ev := <-events:
			h.sendEvent(ctx, ev)
		case <-h.close:
			return
		}
	}
}

func (h *Hub) sendEvent(ctx context.Context, ev *CLIEvent) {
	txn, innerCtx := apm.StartTransaction(ctx, "sendEvent")
	defer txn.End()

	h.lock.RLock()
	client := h.deviceClients[ev.DeviceID]
	h.lock.RUnlock()

	// there is no valid client for this event delivery, so skip it
	if client == nil {
		return
	}

	if !client.IsOnline() {
		client.GoOffline(innerCtx)
		client.Close(unregister)
		return
	}

	if client.Device.ProjectID != ev.ProjectID {
		return
	}

	if !util.IsStringEmpty(client.sourceID) {
		if client.sourceID != ev.SourceID {
			return
		}
	}

	j, err := json.Marshal(ev)
	if err != nil {
		log.WithError(err).Error("failed to marshal cli event")
		return
	}

	err = client.conn.WriteMessage(websocket.BinaryMessage, j)
	if err != nil {
		log.WithError(err).Error("failed to write event to socket")
	}

}

type EventDelivery struct {
	EventDeliveryID string
	ProjectID       string
}

type EndpointError struct {
	delay time.Duration
	Err   error
}

func (e *EndpointError) Error() string {
	return e.Err.Error()
}

func (e *EndpointError) Delay() time.Duration {
	return e.delay
}

func (h *Hub) EventDeliveryCLiHandler(r *Repo) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var data EventDelivery
		err := util.DecodeMsgPack(t.Payload(), &data)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal process event delivery payload")
			return &EndpointError{Err: err, delay: time.Second}
		}

		ed, err := r.EventDeliveryRepo.FindEventDeliveryByID(ctx, data.ProjectID, data.EventDeliveryID)
		if err != nil {
			log.WithError(err).Errorf("Failed to load event - %s", data.EventDeliveryID)
			return &EndpointError{Err: err, delay: time.Second * 5}
		}

		// this isn't a cli event delivery
		if ed.CLIMetadata == nil {
			return nil
		}

		events <- &CLIEvent{
			UID:        ed.UID,
			Data:       ed.Metadata.Data,
			Headers:    ed.Headers,
			EventType:  ed.CLIMetadata.EventType,
			EndpointID: ed.EndpointID,
			DeviceID:   ed.DeviceID,
			SourceID:   ed.CLIMetadata.SourceID,
			ProjectID:  ed.ProjectID,
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

func (h *Hub) StartClientStatusWatcher(ctx context.Context) {
	h.ticker = time.NewTicker(time.Second * 30)
	defer h.ticker.Stop()
	for {
		select {
		case <-h.ticker.C:
			h.checkDeviceStatus(ctx)
		case <-h.close:
			return
		}
	}
}

func (h *Hub) checkDeviceStatus(ctx context.Context) {
	txn, innerCtx := apm.StartTransaction(ctx, "checkDeviceStatus")
	defer txn.End()

	for k, v := range h.deviceClients {
		h.lock.Lock()
		if !h.deviceClients[k].IsOnline() {
			h.deviceClients[k].GoOffline(innerCtx)
			h.deviceClients[k].Close(unregister)
			log.Printf("%s has be set to offline after inactivity for 30 seconds", v.Device.HostName)
		}
		h.lock.Unlock()
	}
}

func (h *Hub) Stop() {
	close(h.close)
}
