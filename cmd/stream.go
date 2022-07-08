package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	m "github.com/frain-dev/convoy/datastore/mongo"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/frain-dev/convoy"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/google/uuid"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/server"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 2 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// device id of the cli client
	deviceID   string
	EventTypes []string

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	deviceClients map[string]*Client

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	deviceRepo       datastore.DeviceRepository
	subscriptionRepo datastore.SubscriptionRepository
	sourceRepo       datastore.SourceRepository

	events chan *CLIEvent
	close  chan struct{}
}

func (h *Hub) StartEventSender() {
	for {
		select {
		case ev := <-h.events:
			client := h.deviceClients[ev.DeviceID] //TODO: protect h.deviceClients against data race

			err := client.conn.WriteMessage(websocket.TextMessage, ev.Data)
			if err != nil {
				log.WithError(err).Error("failed to write pong message")
			}

		case <-h.close:

		}
	}
}

func (h *Hub) StartEventWatcher() {
	lookupStage := bson.D{
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

	fn := h.watchEventDeliveriesCollection()
	err := watchCollection(fn, mongo.Pipeline{lookupStage, matchStage}, m.EventCollection, h.close)
	if err != nil {
		log.WithError(err).Error("database collection watcher exited unexpectedly")
	}
}

type CLIEvent struct {
	Data     json.RawMessage
	DeviceID string
	AppID    string
	GroupID  string
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

		subscription, ok := doc["metadata"].(*datastore.Subscription)
		if !ok {
			return fmt.Errorf("event delivery subscription has wrong type of: %T", doc["metadata"])
		}

		h.events <- &CLIEvent{
			Data:     metadata.Data,
			AppID:    appID,
			DeviceID: subscription.DeviceID,
			GroupID:  groupID,
		}

		return nil
	}
}

func (h *Hub) StartRegister() {
	for {
		select {
		case client := <-h.register:
			h.deviceClients[client.deviceID] = client
		case <-h.close:
			return
		}
	}
}

func (h *Hub) Stop() {
	close(h.close)
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
		ctx, cancel := getCtx()
		defer cancel()

		err := c.hub.deviceRepo.UpdateDeviceLastSeen(ctx, deviceID)
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
					log.Printf("error: %v", err)
				}
				break
			}

		}
	}
}

func addStreamCommand(a *app) *cobra.Command {
	var cronspec string
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "schedule a periodic task.",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Get()
			if err != nil {
				log.Fatalf("Error getting config: %v", err)
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.WithError(err).Fatalf("Queue type error: Command is available for redis queue only.")
			}
			ctx := context.Background()

			//initialize scheduler
			s := worker.NewScheduler(a.queue)

			s.Start()

			router := chi.NewRouter()
			router.Handle("/queue/monitoring/*", a.queue.(*redisqueue.RedisQueue).Monitor())
			router.Handle("/metrics", promhttp.HandlerFor(server.Reg, promhttp.HandlerOpts{}))

			srv := &http.Server{
				Handler: router,
				Addr:    fmt.Sprintf(":%d", 5007),
			}

			e := srv.ListenAndServe()
			if e != nil {
				log.Fatal(e)
			}
			<-ctx.Done()
		},
	}

	cmd.Flags().StringVar(&cronspec, "cronspec", "", "scheduler time interval '@every <duration>'")
	return cmd
}

type DeviceRegistration struct {
	HostName   string   `json:"host_name"`
	DeviceID   string   `json:"device_id"`
	SourceID   string   `json:"source_id"`
	EventTypes []string `json:"event_types"`
}

func (h *Hub) Listen(w http.ResponseWriter, r *http.Request) {
	deviceReg := &DeviceRegistration{}
	err := util.ReadJSON(r, &deviceReg)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "device id is required in request body")
		return
	}

	ctx, cancel := getCtx()
	defer cancel()

	device, err := h.deviceRepo.FetchDeviceByID(ctx, deviceReg.DeviceID)
	if err != nil {
		if errors.Is(err, datastore.ErrDeviceNotFound) {
			device = &datastore.Device{
				UID:        deviceReg.DeviceID,
				HostName:   "",
				Status:     "online",
				LastSeenAt: 0,
				CreatedAt:  primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:  primitive.NewDateTimeFromTime(time.Now()),
			}

			err = h.deviceRepo.CreateDevice(ctx, device)
			if err != nil {
				respondWithError(w, http.StatusBadRequest, "failed to create new device")
				return
			}
		} else {
			respondWithError(w, http.StatusBadRequest, "device id is required in request body")
			return
		}
	}

	if !util.IsStringEmpty(deviceReg.SourceID) {
		_, err = h.sourceRepo.FindSourceByID(ctx, "", deviceReg.SourceID)
		if err != nil {
			log.WithError(err).Error("error retrieving source")
			respondWithError(w, http.StatusBadRequest, "failed to find source")
			return
		}
	}

	subscription, err := h.subscriptionRepo.FindSubscriptionByID(ctx, device.UID, "")
	if err != nil {
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			subscription = &datastore.Subscription{
				UID:      uuid.NewString(),
				DeviceID: device.UID,
				GroupID:  "group.UID",
				Type:     datastore.SubscriptionTypeCLI,
				Name:     fmt.Sprintf("device-%s-subscription", device.UID),
				SourceID: deviceReg.SourceID,
				AppID:    "",

				Status:         datastore.ActiveSubscriptionStatus,
				DocumentStatus: datastore.ActiveDocumentStatus,
				CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			}

			err = h.subscriptionRepo.CreateSubscription(ctx, "", subscription)
			if err != nil {
				respondWithError(w, http.StatusBadRequest, "failed to create new subscription")
				return
			}
		} else {
			respondWithError(w, http.StatusBadRequest, "failed to find subscription by id")
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.WithError(err).Error("failed to upgrade connection to websocket connection")
		respondWithError(w, http.StatusBadRequest, "failed to upgrade connection to websocket connection")
		return
	}

	client := &Client{
		hub:        h,
		conn:       conn,
		deviceID:   device.UID,
		EventTypes: deviceReg.EventTypes,
		send:       make(chan []byte, 256),
	}

	if len(client.EventTypes) == 0 {
		client.EventTypes = []string{"*"}
	}

	client.hub.register <- client
}

func respondWithError(w http.ResponseWriter, code int, err string) {
	w.WriteHeader(code)
	w.Write([]byte(err))
}
