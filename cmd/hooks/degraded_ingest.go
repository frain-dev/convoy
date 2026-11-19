package hooks

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/ingestidentity"
	"github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

func connectStartupPostgres(cmd *cobra.Command, cfg config.Configuration, log logger.Logger) (*postgres.Postgres, error) {
	connect := func() (*postgres.Postgres, error) { return postgres.NewDB(cfg) }
	db, err := connect()
	if err == nil || len(cfg.IngestDeliveryIDHeaders) == 0 || !postgresUnavailable(err) {
		return db, err
	}

	httpConfig := cfg.Server.HTTP
	command := cmd.Name()
	switch command {
	case "server":
		// The normal server applies nonzero CLI overrides after the startup hook.
		ssl, _ := cmd.Flags().GetBool("ssl")
		httpConfig.SSL = httpConfig.SSL || ssl
		if cert, _ := cmd.Flags().GetString("ssl-cert-file"); cert != "" {
			httpConfig.SSLCertFile = cert
		}
		if key, _ := cmd.Flags().GetString("ssl-key-file"); key != "" {
			httpConfig.SSLKeyFile = key
		}
	case "agent":
		httpConfig.Port = httpConfig.AgentPort
	default:
		return db, err
	}
	if port, _ := cmd.Flags().GetUint32("port"); port != 0 {
		httpConfig.Port = port
	}

	return waitForStartupPostgres(cmd.Context(), httpConfig, degradedIngestHandler(cfg, command == "server", log), connect, log)
}

func postgresUnavailable(err error) bool {
	var serverError *pgconn.PgError
	if errors.As(err, &serverError) {
		return strings.HasPrefix(serverError.Code, "08") || serverError.Code == "57P03"
	}
	var networkError net.Error
	return errors.As(err, &networkError) || errors.Is(err, context.DeadlineExceeded)
}

func waitForStartupPostgres(
	ctx context.Context, cfg config.HTTPServerConfiguration, handler http.Handler,
	connect func() (*postgres.Postgres, error), log logger.Logger,
) (*postgres.Postgres, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("listen for webhooks while Postgres is unavailable: %w", err)
	}
	defer listener.Close()
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 10 * time.Second}
	defer server.Close()
	serveErr := make(chan error, 1)
	go func() {
		if cfg.SSL {
			serveErr <- server.ServeTLS(listener, cfg.SSLCertFile, cfg.SSLKeyFile)
			return
		}
		serveErr <- server.Serve(listener)
	}()
	log.Warn("waiting for Postgres with webhook ingestion unavailable")

	retry := time.NewTicker(2 * time.Second)
	defer retry.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-serveErr:
			return nil, fmt.Errorf("serve degraded webhook ingestion: %w", err)
		case <-retry.C:
			db, err := connect()
			if err == nil || !postgresUnavailable(err) {
				return db, err
			}
		}
	}
}

func degradedIngestHandler(cfg config.Configuration, mountUnprefixed bool, log logger.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	rejectWebhook := func(w http.ResponseWriter, req *http.Request) {
		maskID := chi.URLParam(req, "maskID")
		requestID := middleware.GetReqID(req.Context())
		if deliveryID := ingestidentity.DeliveryID(cfg.IngestDeliveryIDHeaders, maskID, req); deliveryID != "" {
			log.Error("webhook ingestion rejected", "request_id", requestID, "upstream_delivery_id", deliveryID,
				"http_status", http.StatusServiceUnavailable, "reason", "Postgres unavailable during startup")
		}
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("Retry-After", "2")
		_ = render.Render(w, req, util.NewErrorResponse("Service temporarily unavailable", http.StatusServiceUnavailable))
	}
	const ingestPath = "/ingest/{maskID}"
	root := strings.TrimRight(cfg.RootPath, "/")
	r.Post(root+ingestPath, rejectWebhook)
	if root != "" && mountUnprefixed {
		r.Post(ingestPath, rejectWebhook)
	}
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
	})
	return r
}
