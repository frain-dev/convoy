package fflag

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag/flipt"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

type IsEnabledFunc func(r *http.Request) error

var (
	ErrFeatureNotAvailable = errors.New("this feature is not yet available for you")
	CanCreateCLIAPIKey     = "can_create_cli_api_key"
)

const (
	fliptProvider = "flipt"
)

func newFliptClient(cfg config.Configuration) *flipt.Flipt {
	if cfg.FeatureFlag.Type == config.FeatureFlagProvider(fliptProvider) {
		client := flipt.NewFliptClient(cfg.FeatureFlag.Flipt.Host)
		return client
	}

	return nil
}

var Features = map[string]IsEnabledFunc{
	CanCreateCLIAPIKey: func(r *http.Request) error {
		group := middleware.GetGroupFromContext(r.Context())

		cfg, err := config.Get()
		if err != nil {
			return err
		}

		ff := newFliptClient(cfg)

		if ff == nil {
			return nil
		}

		var apiKey models.CreateEndpointApiKey
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return err
		}

		// Replace the body with a new reader after reading from the original
		// request
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		err = json.Unmarshal(body, &apiKey)
		if err != nil {
			return err
		}

		if apiKey.KeyType == datastore.CLIKey {
			isEnabled, err := ff.IsEnabled(CanCreateCLIAPIKey, map[string]string{
				"group_id":        group.UID,
				"organisation_id": group.OrganisationID,
			})

			if err != nil {
				log.WithError(err).Error("failed to check flag on flipt")
				return err
			}

			if !isEnabled {
				return ErrFeatureNotAvailable
			}
		}

		return nil
	},
}

func CanAccessFeature(fn IsEnabledFunc) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := fn(r)

			if err != nil {
				statusCode := http.StatusInternalServerError

				if errors.Is(err, flipt.ErrFliptServerError) {
					statusCode = http.StatusInternalServerError
				}

				if errors.Is(err, flipt.ErrFliptFlagNotFound) {
					statusCode = http.StatusNotFound
				}

				if errors.Is(err, ErrFeatureNotAvailable) {
					statusCode = http.StatusForbidden
				}

				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), statusCode))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
