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

func NewClient(conn WebSocketConnection, device *datastore.Device, sourceID string, deviceRepo datastore.DeviceRepository, eventDeliveryRepo datastore.EventDeliveryRepository) {
	client := &Client{
		conn:              conn,
		Device:            device,
		deviceID:          device.UID,
		sourceID:          sourceID,
		deviceRepo:        deviceRepo,
		eventDeliveryRepo: eventDeliveryRepo,
	}

	register <- client
	go client.readPump(unregister)
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump(unregister chan *Client) {
	defer c.Close(unregister)

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPingHandler(c.pingHandler)

	for {
		messageType, message, err := c.conn.ReadMessage()
		c.processMessage(messageType, message, err, unregister)

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.WithError(err).WithField("device_id", c.deviceID).Error("unexpected close error")
			}
			return
		}
	}
}

func (c *Client) processMessage(messageType int, message []byte, err error, unregister chan *Client) {
	// fmt.Printf("type: %+v \nmess: %+v \nerr: %+v\n", messageType, message, err)

	// messageType -1 means an error occured
	// set the device of this client to offline
	if messageType == -1 {
		c.GoOffline()
	}

	if messageType == websocket.CloseMessage {
		c.Close(unregister)
	}

	if messageType == websocket.TextMessage {
		// this is triggered when a SIGINT signal (Ctrl + C) is sent by the client
		if string(message) == "disconnect" {
			c.GoOffline()
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

			go c.ResendEventDeliveries(since, events)
			return
		}

		var ed AckEventDelivery
		err := json.Unmarshal(message, &ed)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal text message")
			return
		}
		go c.UpdateEventDeliveryStatus(ed.UID)
	}
}

func (c *Client) pingHandler(appData string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	err := c.deviceRepo.UpdateDeviceLastSeen(context.Background(), c.Device, c.Device.EndpointID, c.Device.ProjectID, datastore.DeviceStatusOnline)
	if err != nil {
		log.WithError(err).Error(ErrFailedToUpdateDevice.Error())
		return ErrFailedToUpdateDevice
	}

	err = c.conn.WriteMessage(websocket.PongMessage, []byte("ok"))
	if err != nil {
		log.WithError(err).Error(ErrFailedToSendPongMessage.Error())
		return ErrFailedToSendPongMessage
	}

	return nil
}

func (c *Client) Close(unregister chan *Client) {
	err := c.conn.Close()
	if err != nil {
		log.WithError(err).Error("failed to close client conn")
	}
	unregister <- c
}

func (c *Client) GoOffline() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.Device.Status = datastore.DeviceStatusOffline

	err := c.deviceRepo.UpdateDevice(context.Background(), c.Device, c.Device.EndpointID, c.Device.ProjectID)
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
	var since time.Time = time.Now()
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

func (c *Client) UpdateEventDeliveryStatus(id string) {
	ed, err := c.eventDeliveryRepo.FindEventDeliveryByID(context.Background(), id)
	if err != nil {
		log.WithError(err).WithField("event_delivery_id", id).Error("failed to find event delivery")
	}

	err = c.eventDeliveryRepo.UpdateStatusOfEventDelivery(context.Background(), *ed, datastore.SuccessEventStatus)
	if err != nil {
		log.WithError(err).WithField("event_delivery_id", id).Error("failed to update event delivery status")
	}
}

func (c *Client) ResendEventDeliveries(since time.Time, events chan *CLIEvent) {
	eds, err := c.eventDeliveryRepo.FindDiscardedEventDeliveries(context.Background(), c.Device.EndpointID, c.deviceID,
		datastore.SearchParams{CreatedAtStart: since.Unix(), CreatedAtEnd: time.Now().Unix()})
	if err != nil {
		log.WithError(err).Error("failed to find discarded event deliveries")
	}

	if eds == nil {
		return
	}

	for _, ed := range eds {
		events <- &CLIEvent{
			UID:        ed.UID,
			Data:       ed.Metadata.Data,
			Headers:    ed.Headers,
			EventType:  ed.CLIMetadata.EventType,
			EndpointID: ed.EndpointID,
			DeviceID:   ed.DeviceID,
			ProjectID:  ed.ProjectID,
		}
	}
}
