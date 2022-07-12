package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/frain-dev/convoy/server"

	"github.com/frain-dev/convoy/auth/realm_chain"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/google/uuid"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
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

	maxDeviceLastSeenDuration = 2 * time.Minute
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func addStreamCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Start a websocket server to pipe events to another convoy instance",
		Run: func(cmd *cobra.Command, args []string) {
			// enable only the native auth realm
			authCfg := &config.AuthConfiguration{
				Native: config.NativeRealmOptions{Enabled: true},
			}
			err := realm_chain.Init(authCfg, a.apiKeyRepo, nil, nil)
			if err != nil {
				log.WithError(err).Fatal("failed to initialize realm chain")
			}

			hub := NewHub(a.deviceRepo, a.subRepo, a.sourceRepo)
			go hub.StartRegister()
			go hub.StartEventWatcher()
			go hub.StartEventSender()

			router := chi.NewRouter()
			router.Route("/stream", func(streamRouter chi.Router) {
				streamRouter.Use(server.RequireAuth())
				streamRouter.Use(server.RequireGroup(a.groupRepo, a.cache))
				streamRouter.Use(hub.requireApp())

				streamRouter.Get("/listen", hub.Listen)
				streamRouter.Post("/login", hub.Login)
			})

			srv := &http.Server{
				Handler: router,
				Addr:    fmt.Sprintf(":%d", 5008),
			}

			go func() {
				//service connections
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.WithError(err).Fatal("failed to listen")
				}
			}()

			gracefulShutdown(srv, hub)
		},
	}

	return cmd
}

func gracefulShutdown(srv *http.Server, hub *Hub) {
	//Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	hub.Stop()

	log.Info("Stopping websocket server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.WithError(err).Fatal("Server Shutdown")
	}

	log.Info("Websocket server exiting")

	time.Sleep(2 * time.Second) // allow all websocket connections close themselves
}

type ListenRequest struct {
	HostName   string   `json:"host_name"`
	DeviceID   string   `json:"device_id"`
	SourceID   string   `json:"source_id"`
	EventTypes []string `json:"event_types"`
}

type LoginRequest struct {
	HostName string `json:"host_name"`
}

func (h *Hub) Login(w http.ResponseWriter, r *http.Request) {
	loginRequest := &LoginRequest{}
	err := util.ReadJSON(r, &loginRequest)
	if err != nil {
		respond(w, http.StatusBadRequest, "device id is required in request body")
		return
	}

	group := server.GetGroupFromContext(r.Context())
	app, ok := getApplicationFromContext(r.Context())

	appID := ""
	if ok {
		appID = app.UID
	}

	device := &datastore.Device{
		UID:        uuid.NewString(),
		GroupID:    group.UID,
		AppID:      appID,
		HostName:   loginRequest.HostName,
		Status:     "online",
		LastSeenAt: primitive.NewDateTimeFromTime(time.Now()),
		CreatedAt:  primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:  primitive.NewDateTimeFromTime(time.Now()),
	}

	ctx, cancel := getCtx()
	defer cancel()

	err = h.deviceRepo.CreateDevice(ctx, device, group.UID, appID)
	if err != nil {
		respond(w, http.StatusBadRequest, "failed to create new device")
		return
	}

	respondWithData(w, http.StatusOK, device)
}

func (h *Hub) Listen(w http.ResponseWriter, r *http.Request) {
	listenRequest := &ListenRequest{}
	err := util.ReadJSON(r, &listenRequest)
	if err != nil {
		respond(w, http.StatusBadRequest, "empty request body")
		return
	}

	group := server.GetGroupFromContext(r.Context())
	app, ok := getApplicationFromContext(r.Context())

	appID := ""
	if ok {
		appID = app.UID
	}

	ctx, cancel := getCtx()
	defer cancel()

	device, err := h.deviceRepo.FetchDeviceByID(ctx, listenRequest.DeviceID, appID, group.UID)
	if err != nil {
		respond(w, http.StatusBadRequest, "device not found")
		return
	}

	if device.GroupID != group.UID {
		respond(w, http.StatusUnauthorized, "unauthorized to access device")
		return
	}

	if device.AppID != appID {
		respond(w, http.StatusUnauthorized, "unauthorized to access device")
		return
	}

	if !util.IsStringEmpty(listenRequest.SourceID) {
		source, err := h.sourceRepo.FindSourceByID(ctx, "", listenRequest.SourceID)
		if err != nil {
			log.WithError(err).Error("error retrieving source")
			respond(w, http.StatusBadRequest, "failed to find source")
			return
		}

		if source.GroupID != group.UID {
			respond(w, http.StatusUnauthorized, "unauthorized to access source")
			return
		}
	}

	subscription, err := h.subscriptionRepo.FindSubscriptionByID(ctx, group.UID, device.UID)

	switch err {
	case nil:
		break
	case datastore.ErrSubscriptionNotFound:
		subscription = &datastore.Subscription{
			UID:            uuid.NewString(),
			DeviceID:       device.UID,
			GroupID:        "group.UID",
			Type:           datastore.SubscriptionTypeCLI,
			Name:           fmt.Sprintf("device-%s-subscription", device.UID),
			SourceID:       listenRequest.SourceID,
			Status:         datastore.ActiveSubscriptionStatus,
			DocumentStatus: datastore.ActiveDocumentStatus,
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		}

		if ok {
			subscription.AppID = app.UID
		}

		err := h.subscriptionRepo.CreateSubscription(ctx, "", subscription)
		if err != nil {
			respond(w, http.StatusBadRequest, "failed to create new subscription")
			return
		}
	default:
		respond(w, http.StatusBadRequest, "failed to find subscription by id")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.WithError(err).Error("failed to upgrade connection to websocket connection")
		respond(w, http.StatusBadRequest, "failed to upgrade connection to websocket connection")
		return
	}

	client := &Client{
		hub:        h,
		conn:       conn,
		deviceID:   device.UID,
		EventTypes: listenRequest.EventTypes,
		send:       make(chan []byte, 256),
	}

	if len(client.EventTypes) == 0 {
		client.EventTypes = []string{"*"}
	}

	client.hub.register <- client
	go client.readPump()
}

func respond(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	w.Write([]byte(msg))
}

func respondWithData(w http.ResponseWriter, code int, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.WithError(err).Error("failed to marshal data")
		respond(w, http.StatusInternalServerError, "failed to marshal response")
		return
	}

	w.WriteHeader(code)
	w.Write(data)
}

func (h *Hub) requireApp() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser := server.GetAuthUserFromContext(r.Context())
			if len(authUser.Role.Apps) > 0 {
				appID := authUser.Role.Apps[0]
				app, err := h.appRepo.FindApplicationByID(r.Context(), appID)
				if err != nil {
					respond(w, http.StatusBadRequest, "failed to find application")
					return
				}

				r = r.WithContext(server.SetApplicationInContext(r.Context(), app))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// the app may not exist, so we have to check like this to avoid panic
func getApplicationFromContext(ctx context.Context) (*datastore.Application, bool) {
	app, ok := ctx.Value("app").(*datastore.Application)
	return app, ok
}
