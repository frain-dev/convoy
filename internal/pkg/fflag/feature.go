package fflag

import (
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag/flipt"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

type IsEnabledFunc func(r *http.Request, ff FeatureFlag) error

var (
	CanCreateCLIAPIKey = "can_create_cli_api_key"
)

var Features = map[string]IsEnabledFunc{
	CanCreateCLIAPIKey: func(r *http.Request, ff FeatureFlag) error {
		group := middleware.GetGroupFromContext(r.Context())

		var apiKey models.CreateAppApiKey
		if err := util.ReadJSON(r, &apiKey); err != nil {
			return err
		}

		if apiKey.KeyType == datastore.CLIKey {
			isEnabled, err := ff.IsEnabled(CanCreateCLIAPIKey, map[string]string{
				"group_id":        group.UID,
				"organisation_id": group.OrganisationID,
			})

			if err != nil {
				return err
			}

			if !isEnabled {
				return errors.New("this feature is not yet avaiable for you, sorry")
			}
		}

		return nil
	},
}

func CanAccessFeature(ff FeatureFlag, fn IsEnabledFunc) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := fn(r, ff)

			if err != nil {
				statusCode := http.StatusForbidden

				if errors.Is(err, flipt.ErrFliptServerError) {
					statusCode = http.StatusInternalServerError
				}

				if errors.Is(err, flipt.ErrFliptFlagNotFound) {
					statusCode = http.StatusNotFound
				}

				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), statusCode))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
