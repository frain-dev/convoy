package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"time"

	"github.com/frain-dev/convoy/config"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type Server struct {
	s            *http.Server
	CACertTLSCfg *tls.Config
	StopFn       func()
}

func NewServer(port uint32, caCertPath string, stopFn func()) (*Server, error) {
	caCertTLSCfg, err := GetCACertTLSCfg(caCertPath)
	if err != nil {
		return nil, err
	}

	srv := &Server{
		s: &http.Server{
			ReadTimeout:  time.Second * 30,
			WriteTimeout: time.Second * 30,
			Addr:         fmt.Sprintf(":%d", port),
		},
		CACertTLSCfg: caCertTLSCfg,
		StopFn:       stopFn,
	}

	return srv, nil
}

func (s *Server) SetHandler(handler http.Handler) {
	router := chi.NewRouter()
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("Convoy %v", convoy.GetVersion()), nil, http.StatusOK))
	})

	cfg, err := config.Get()
	if err != nil {
		log.WithError(err).Fatal("failed to start server")
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

	router.Handle("/*", handler)
	s.s.Handler = router
}

func (s *Server) SetStopFunction(fn func()) {
	s.StopFn = fn
}

func (s *Server) Listen() {
	go func() {
		//service connections
		err := s.s.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.WithError(err).Fatal("failed to listen")
		}
	}()

	s.gracefulShutdown()
}

func (s *Server) ListenAndServeTLS(certFile, keyFile string) {
	go func() {
		//service connections
		err := s.s.ListenAndServeTLS(certFile, keyFile)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.WithError(err).Fatal("failed to listen")
		}
	}()

	s.gracefulShutdown()
}

func (s *Server) gracefulShutdown() {
	//Wait for interrupt signal to gracefully shut down the server with a timeout of 10 seconds
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Info("Stopping server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.s.Shutdown(ctx); err != nil {
		log.WithError(err).Fatal("Server Shutdown")
	}

	log.Info("Server exiting")

	time.Sleep(2 * time.Second) // allow all pending connections to close themselves
}

func GetCACertTLSCfg(caCertPath string) (*tls.Config, error) {
	var tlsCfg *tls.Config
	if caCertPath != "" {
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
		tlsCfg = &tls.Config{
			RootCAs:    caCertPool,
			MinVersion: tls.VersionTLS12,
		}
	}
	return tlsCfg, nil
}
