package socket

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/gorilla/websocket"
)

const (
	// Maximum message size allowed from peer.
	maxMessageSize = 512

	maxDeviceLastSeenDuration = 10 * time.Second
)

var (
	ErrFailedToUpdateDevice    = errors.New("failed to update device last seen")
	ErrFailedToSendPongMessage = errors.New("failed to write pong message")
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	deviceID          string
	sourceID          string
	Device            *datastore.Device
	deviceRepo        datastore.DeviceRepository
	eventDeliveryRepo datastore.EventDeliveryRepository

	// The websocket connection.
	conn WebSocketConnection

	lock sync.RWMutex // protect Device from data race
}

func NewClient(ctx context.Context, conn WebSocketConnection, device *datastore.Device, sourceID string, deviceRepo datastore.DeviceRepository, eventDeliveryRepo datastore.EventDeliveryRepository) {
	client := &Client{
		conn:              conn,
		Device:            device,
		deviceID:          device.UID,
		sourceID:          sourceID,
		deviceRepo:        deviceRepo,
		eventDeliveryRepo: eventDeliveryRepo,
	}

	register <- client
	go client.readPump(ctx, unregister)
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump(ctx context.Context, unregister chan *Client) {
	defer c.Close(unregister)

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPingHandler(c.pingHandler(ctx))

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.WithError(err).WithField("device_id", c.deviceID).Error("unexpected close error")
			}
			return
		}

		c.processMessage(ctx, messageType, message, unregister)
	}
}

func (c *Client) processMessage(ctx context.Context, messageType int, message []byte, unregister chan *Client) {
	// messageType -1 means an error occurred
	// set the device of this client to offline
	if messageType == -1 {
		c.GoOffline(ctx)
	}

	if messageType == websocket.CloseMessage {
		c.Close(unregister)
	}

	if messageType == websocket.TextMessage {
		// this is triggered when a SIGINT signal (Ctrl + C) is sent by the client
		if string(message) == "disconnect" {
			c.GoOffline(ctx)
			return
		}

		// the "since" message is formatted thus:
		// "since|duration|2m"
		// "since|timestamp|2022-08-31T00:00:00Z"
		if strings.HasPrefix(string(message), "since") {
			since, err := c.parseTime(string(message))
			if err != nil {
				log.Error(err)
			}

			go c.ResendEventDeliveries(ctx, since, events)
			return
		}

		var ed AckEventDelivery
		err := json.Unmarshal(message, &ed)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal text message")
			return
		}
		go c.UpdateEventDeliveryStatus(ctx, ed.UID, c.Device.ProjectID)
	}
}

func (c *Client) pingHandler(ctx context.Context) func(appData string) error {
	return func(appData string) error {
		c.lock.Lock()
		defer c.lock.Unlock()

		err := c.deviceRepo.UpdateDeviceLastSeen(ctx, c.Device, c.Device.EndpointID, c.Device.ProjectID, datastore.DeviceStatusOnline)
		if err != nil {
			log.WithError(err).Error(ErrFailedToUpdateDevice.Error())
			return ErrFailedToUpdateDevice
		}

		c.Device.LastSeenAt = time.Now()

		err = c.conn.WriteMessage(websocket.PongMessage, []byte("ok"))
		if err != nil {
			log.WithError(err).Error(ErrFailedToSendPongMessage.Error())
			return ErrFailedToSendPongMessage
		}

		return nil
	}
}

func (c *Client) Close(unregister chan *Client) {
	err := c.conn.Close()
	if err != nil {
		log.WithError(err).Error("failed to close client conn")
	}
	unregister <- c
}

func (c *Client) GoOffline(ctx context.Context) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.Device.Status = datastore.DeviceStatusOffline

	err := c.deviceRepo.UpdateDevice(ctx, c.Device, c.Device.EndpointID, c.Device.ProjectID)
	if err != nil {
		log.WithError(err).Error("failed to update device status to offline")
	}
}

func (c *Client) IsOnline() bool {
	c.lock.RLock()
	lastSeen := c.Device.LastSeenAt
	c.lock.RUnlock()

	since := time.Since(lastSeen)
	return since < maxDeviceLastSeenDuration
}

func (c *Client) parseTime(message string) (time.Time, error) {
	var since = time.Now()
	var err error

	timeType := strings.Split(message, "|")[1]
	timeStr := strings.Split(message, "|")[2]

	switch timeType {
	case "timestamp":
		since, err = time.Parse(time.RFC3339, timeStr)
		if err != nil {
			log.WithError(err).Error("'since' is not a valid timestamp, will ignore it")
			return since, err
		}
	case "duration":
		dur, err := time.ParseDuration(timeStr)
		if err != nil {
			log.WithError(err).Error("'since' is not a valid time duration, will ignore it")
			return since, err
		} else {
			since = time.Now().Add(-dur)
		}
	default:
		log.Error("will ignore 'since' value")
		return since, errors.New("will ignore 'since' value")
	}

	return since, nil
}

func (c *Client) UpdateEventDeliveryStatus(ctx context.Context, id, projectId string) {
	ed, err := c.eventDeliveryRepo.FindEventDeliveryByID(ctx, projectId, id)
	if err != nil {
		log.WithError(err).WithField("event_delivery_id", id).Error("failed to find event delivery")
	}

	err = c.eventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, c.Device.ProjectID, *ed, datastore.SuccessEventStatus)
	if err != nil {
		log.WithError(err).WithField("event_delivery_id", id).Error("failed to update event delivery status")
	}
}

func (c *Client) ResendEventDeliveries(ctx context.Context, since time.Time, events chan *CLIEvent) {
	eds, err := c.eventDeliveryRepo.FindDiscardedEventDeliveries(ctx, c.Device.ProjectID, c.Device.UID,
		datastore.SearchParams{CreatedAtStart: since.Unix(), CreatedAtEnd: time.Now().Unix()})
	if err != nil {
		log.WithError(err).Error("failed to find discarded event deliveries")
	}

	if len(eds) == 0 {
		return
	}

	for _, ed := range eds {
		events <- &CLIEvent{
			UID:        ed.UID,
			Data:       ed.Metadata.Data,
			Headers:    ed.Headers,
			EventType:  ed.CLIMetadata.EventType,
			EndpointID: ed.EndpointID,
			SourceID:   ed.CLIMetadata.SourceID,
			DeviceID:   ed.DeviceID,
			ProjectID:  ed.ProjectID,
		}
	}
}
