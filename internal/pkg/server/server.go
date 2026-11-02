package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

type Server struct {
	s      *http.Server
	StopFn func()
	logger log.Logger
}

func NewServer(port uint32, stopFn func()) *Server {
	return NewServerWithLogger(port, stopFn, log.New("server", log.LevelInfo))
}

func NewServerWithLogger(port uint32, stopFn func(), logger log.Logger) *Server {
	// The write deadline starts when the request headers are done, before the
	// handler reads the body, and the response still has to be written after
	// the body. Sync-ack waits on POST /events/dynamic need the same headroom.
	readTimeout := 30 * time.Second
	writeTimeout := 30 * time.Second
	if cfg, err := config.Get(); err == nil {
		readTimeout, writeTimeout = httpServerTimeouts(cfg.Server.HTTP.ReadTimeout, cfg.VerifyDynamicEventsTimeout)
	}
	headerTimeout := 30 * time.Second
	if readTimeout < headerTimeout {
		headerTimeout = readTimeout
	}

	srv := &Server{
		s: &http.Server{
			ReadTimeout: readTimeout,
			// A longer body limit must not extend the header wait. A shorter one
			// caps headers too, so that wait cannot outlive the request.
			ReadHeaderTimeout: headerTimeout,
			WriteTimeout:      writeTimeout,
			// Keep-alive wait stays at the previous 30s instead of following ReadTimeout.
			IdleTimeout: 30 * time.Second,
			Addr:        fmt.Sprintf(":%d", port),
		},
		StopFn: stopFn,
		logger: logger,
	}

	return srv
}

func httpServerTimeouts(readSeconds, ackSeconds uint64) (time.Duration, time.Duration) {
	if readSeconds == 0 {
		readSeconds = 30
	}
	if ackSeconds == 0 {
		ackSeconds = 30
	}
	readTimeout := time.Duration(readSeconds) * time.Second
	// The write deadline is already running while the body is read, and the
	// response still has to be written after that.
	writeTimeout := readTimeout + 15*time.Second
	if needed := time.Duration(ackSeconds)*time.Second + 15*time.Second; needed > writeTimeout {
		writeTimeout = needed
	}
	return readTimeout, writeTimeout
}

func (s *Server) SetHandler(handler http.Handler) {
	router := chi.NewRouter()
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("Convoy %v", convoy.GetVersion()), nil, http.StatusOK))
	})

	cfg, err := config.Get()
	if err != nil {
		s.logger.Error("failed to start server", "error", err)
	}

	if cfg.EnableProfiling {
		router.Route("/debug", func(pprofRouter chi.Router) {
			pprofRouter.HandleFunc("/pprof/", pprof.Index)
			pprofRouter.HandleFunc("/pprof/cmdline", pprof.Cmdline)
			pprofRouter.HandleFunc("/pprof/profile", pprof.Profile)
			pprofRouter.HandleFunc("/pprof/symbol", pprof.Symbol)
			pprofRouter.HandleFunc("/pprof/trace", pprof.Trace)

			pprofRouter.Handle("/pprof/goroutine", pprof.Handler("goroutine"))
		})
	}

	router.Mount("/", handler)
	s.s.Handler = router
}

func (s *Server) SetStopFunction(fn func()) {
	s.StopFn = fn
}

func (s *Server) Listen() {
	go func() {
		// serve connections
		err := s.s.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("failed to listen", "error", err)
		}
	}()

	s.gracefulShutdown()
}

func (s *Server) ListenAndServeTLS(certFile, keyFile string) {
	go func() {
		// serve connections
		err := s.s.ListenAndServeTLS(certFile, keyFile)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("failed to listen", "error", err)
		}
	}()

	s.gracefulShutdown()
}

func (s *Server) gracefulShutdown() {
	//Wait for interrupt signal to gracefully shut down the server with a timeout of 10 seconds
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	s.logger.Info("Stopping server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.s.Shutdown(ctx); err != nil {
		s.logger.Error("Server Shutdown", "error", err)
	}

	s.logger.Info("Server exiting")

	time.Sleep(2 * time.Second) // allow all pending connections to close themselves
}
