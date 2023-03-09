package domain

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/server"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/config"
	"github.com/spf13/cobra"
)

func AddDomainCommand(a *cli.App) *cobra.Command {
	var domainPort uint32
	var logLevel string
	var allowedRoutes = []string{
		"ingest",
	}

	cmd := &cobra.Command{
		Use:   "domain",
		Short: "Start a server that forwards requests from a custom domain",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Get()
			if err != nil {
				a.Logger.WithError(err).Fatal("failed to load config")
				return err
			}

			orgRepo := postgres.NewOrgRepo(a.DB)

			lo := a.Logger.(*log.Logger)
			lo.SetPrefix("domain server")

			lvl, err := log.ParseLevel(c.Logger.Level)
			if err != nil {
				return err
			}
			lo.SetLevel(lvl)

			if c.Server.HTTP.SocketPort != 0 {
				domainPort = c.Server.HTTP.DomainPort
			}

			s := server.NewServer(domainPort, func() {})
			client := &http.Client{Timeout: time.Second * 10}

			router := chi.NewRouter()
			router.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
				rElems := strings.Split(r.URL.Path, "/")

				_, err := orgRepo.FetchOrganisationByCustomDomain(r.Context(), r.Host)
				if err != nil {
					// custom domain is not found try the assigned domain
					_, err = orgRepo.FetchOrganisationByAssignedDomain(r.Context(), r.Host)
					if err != nil {
						_ = render.Render(w, r, util.NewErrorResponse("Invalid domain", http.StatusBadRequest))
						return
					}
				}

				if ok := contains(allowedRoutes, rElems[1]); !ok {
					_ = render.Render(w, r, util.NewErrorResponse("Cannot access this route using a custom domain", http.StatusBadRequest))
					return
				}

				forwardedPath := strings.Join(rElems[1:], "/")
				redirectURL := fmt.Sprintf("%s/%s?%s", c.Host, forwardedPath, r.URL.RawQuery)

				redirectReq, err := http.NewRequest(r.Method, redirectURL, r.Body)
				if err != nil {
					log.WithError(err).Error("error occurred while creating the request")
					return
				}

				redirectReq.Header = r.Header

				res, err := client.Do(redirectReq)
				if err != nil {
					log.WithError(err).Error("error occurred while forwarding the request")
					return
				}

				for k, v := range res.Header {
					w.Header().Add(k, v[0])
				}

				w.WriteHeader(res.StatusCode)
				body, err := io.ReadAll(res.Body)
				if err != nil {
					log.WithError(err).Error("error occurred while reading the response body")
					return
				}

				_, err = w.Write(body)
				if err != nil {
					log.WithError(err).Error("error occurred while writing response")
					return
				}
			})

			log.Infof("Domain server running on port %v", domainPort)

			s.SetHandler(router)
			s.Listen()

			return nil
		},
	}

	cmd.Flags().Uint32Var(&domainPort, "domain-port", 5009, "Domain server port")
	cmd.Flags().StringVar(&logLevel, "log-level", "error", "Domain server log level")
	return cmd
}

func contains(sl []string, name string) bool {
	for _, value := range sl {
		if ok := strings.HasPrefix(name, value); ok {
			return true
		}
	}

	return false
}
