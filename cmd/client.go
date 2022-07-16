package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// device id of the cli client
	deviceID   string
	EventTypes []string
	Device     *datastore.Device

	// The websocket connection.
	conn *websocket.Conn

	lock sync.RWMutex // protect Device from data race
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer c.Close()

	c.conn.SetReadLimit(maxMessageSize)

	err := c.conn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		return
	}

	c.conn.SetPongHandler(func(string) error { return nil })

	c.conn.SetPingHandler(func(string) error {
		c.lock.Lock()
		defer c.lock.Unlock()

		err := c.hub.deviceRepo.UpdateDeviceLastSeen(context.Background(), c.Device, c.Device.AppID, c.Device.GroupID)
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
		case <-c.hub.close:
			return
		default:
			_, _, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
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
	c.hub.unregister <- c
}

func (c *Client) GoOffline() {
	c.lock.Lock()

	c.Device.Status = datastore.DeviceStatusOffline

	err := c.hub.deviceRepo.UpdateDevice(context.Background(), c.Device, c.Device.AppID, c.Device.GroupID)
	if err != nil {
		log.WithError(err).Error("failed to update device status to offline")
	}

	c.lock.Unlock()
	c.Close()
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
