package main

import (
	"context"
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

	// Buffered channel of outbound messages.
	send chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	err := c.conn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		return
	}

	c.conn.SetPongHandler(func(string) error {
		return nil
	})

	c.conn.SetPingHandler(func(deviceID string) error {
		err := c.hub.deviceRepo.UpdateDeviceLastSeen(context.Background(), deviceID)
		if err != nil {
			log.WithError(err).Error("failed to update device last seen")
		}

		err = c.conn.WriteMessage(websocket.PongMessage, []byte("ok"))
		if err != nil {
			log.WithError(err).Error("failed to write pong message")
		}
		return nil
	})

	for {
		select {
		case <-c.hub.close:
			err := c.conn.Close()
			if err != nil {
				log.WithError(err).Error("failed to close client conn")
			}
		default:
			_, _, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.WithError(err).
						WithField("device_id", c.deviceID).
						Error("failed to read message from client")
				}
				break // TODO: not confident about breaking the loop which then returns the function because we couldn't read one message from the client here.
			}
		}
	}
}
