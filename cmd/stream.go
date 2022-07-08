package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/google/uuid"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/analytics"
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
	pongWait = 60 * time.Second

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
	deviceID string

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Registered clients.
	deviceClients map[string]*Client

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	deviceRepo       datastore.DeviceRepository
	subscriptionRepo datastore.SubscriptionRepository

	close chan struct{}
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	err := c.conn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		return
	}

	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	c.conn.SetPingHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	<-c.hub.close
	err = c.conn.Close()
	if err != nil {
		log.WithError(err).Error("failed to close client conn")
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

			s.RegisterTask("55 23 * * *", convoy.TaskName("daily analytics"), analytics.TrackDailyAnalytics(&analytics.Repo{
				ConfigRepo: a.configRepo,
				EventRepo:  a.eventRepo,
				GroupRepo:  a.groupRepo,
				OrgRepo:    a.orgRepo,
				UserRepo:   a.userRepo,
			}, cfg))

			// Start scheduler
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
	DeviceID string `json:"device_id"`
	SourceID string `json:"source_id"`
}

func (hub *Hub) Listen(w http.ResponseWriter, r *http.Request) {
	deviceReg := &DeviceRegistration{}
	err := util.ReadJSON(r, &deviceReg)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "device id is required in request body")
		return
	}

	ctx, cancel := getCtx()
	defer cancel()

	device, err := hub.deviceRepo.FetchDeviceByID(ctx, deviceReg.DeviceID)
	if err != nil {
		if errors.Is(err, datastore.ErrDeviceNotFound) {
			device = &datastore.Device{
				UID:        uuid.NewString(),
				HostName:   "",
				Status:     "online",
				LastSeenAt: 0,
				CreatedAt:  primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:  primitive.NewDateTimeFromTime(time.Now()),
			}

			err = hub.deviceRepo.CreateDevice(ctx, device)
			if err != nil {
				respondWithError(w, http.StatusBadRequest, "failed to create new device")
				return
			}
		} else {
			respondWithError(w, http.StatusBadRequest, "device id is required in request body")
			return
		}

		subscription, err := hub.subscriptionRepo.FindSubscriptionsByAppID(ctx, deviceReg.DeviceID)

	}
}

func respondWithError(w http.ResponseWriter, code int, err string) {
	w.WriteHeader(code)
	w.Write([]byte(err))
}
