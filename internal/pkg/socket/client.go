package socket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	// Maximum message size allowed from peer.
	maxMessageSize = 512

	maxDeviceLastSeenDuration = 10 * time.Second
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	// device id of the cli client
	deviceID          string
	EventTypes        []string
	Device            *datastore.Device
	deviceRepo        datastore.DeviceRepository
	eventDeliveryRepo datastore.EventDeliveryRepository

	// The websocket connection.
	conn *websocket.Conn

	lock sync.RWMutex // protect Device from data race
}

func NewClient(hub *Hub, conn *websocket.Conn, device *datastore.Device, events []string, deviceRepo datastore.DeviceRepository, eventDeliveryRepo datastore.EventDeliveryRepository) {
	client := &Client{
		conn:              conn,
		Device:            device,
		EventTypes:        events,
		deviceID:          device.UID,
		deviceRepo:        deviceRepo,
		eventDeliveryRepo: eventDeliveryRepo,
	}

	register <- client
	go client.readPump()
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer c.Close()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPingHandler(func(string) error {
		c.lock.Lock()
		defer c.lock.Unlock()

		err := c.deviceRepo.UpdateDeviceLastSeen(context.Background(), c.Device, c.Device.AppID, c.Device.GroupID, datastore.DeviceStatusOnline)
		if err != nil {
			log.WithError(err).Error("failed to update device last seen")
			return errors.New("failed to update device last seen")
		}

		err = c.conn.WriteMessage(websocket.PongMessage, []byte("ok"))
		if err != nil {
			log.WithError(err).Error("failed to write pong message")
			return errors.New("failed to write pong message")
		}

		return nil
	})

	for {
		select {
		default:
			messageType, message, err := c.conn.ReadMessage()
			fmt.Printf("type: %+v \nmess: %+v \nerr: %+v\n", messageType, message, err)

			// messageType -1 means an error occured
			// set the device of this client to offline
			if messageType == -1 {
				c.GoOffline()
			}

			if messageType == websocket.CloseMessage {
				c.Close()
			}

			if messageType == websocket.TextMessage {
				// this is triggered when a SIGINT signal (Ctrl + C) is sent by the client
				if string(message) == "disconnect" {
					c.GoOffline()
					continue
				}

				var ed AckEventDelivery
				err := json.Unmarshal(message, &ed)
				if err != nil {
					log.WithError(err).Error("failed to unmarshal text message")
					continue
				}
				go c.UpdateEventDeliveryStatus(ed.UID)
			}

			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.WithError(err).WithField("device_id", c.deviceID).Error("unexpected close error")
				}
				return
			}
		}
	}
}

func (c *Client) Close() {
	err := c.conn.Close()
	if err != nil {
		log.WithError(err).Error("failed to close client conn")
	}
	unregister <- c
}

func (c *Client) GoOffline() {
	c.lock.Lock()

	c.Device.Status = datastore.DeviceStatusOffline

	err := c.deviceRepo.UpdateDevice(context.Background(), c.Device, c.Device.AppID, c.Device.GroupID)
	if err != nil {
		log.WithError(err).Error("failed to update device status to offline")
	}

	c.lock.Unlock()
}

func (c *Client) IsOnline() bool {
	c.lock.RLock()
	lastSeen := c.Device.LastSeenAt.Time()
	c.lock.RUnlock()

	since := time.Since(lastSeen)
	return since < maxDeviceLastSeenDuration
}

func (c *Client) HasEventType(evType string) bool {
	for _, eventType := range c.EventTypes {
		if evType == eventType || eventType == "*" {
			return true
		}
	}
	return false
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
