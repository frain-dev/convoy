package fflag

import (
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/internal/pkg/fflag/flipt"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

type IsEnabledFunc func(flagKey string, r *http.Request, ff FeatureFlag) error

//predefined list of features
type Feature struct {
	FlagKey   string
	IsEnabled IsEnabledFunc
}

var Features = []*Feature{
	{
		FlagKey: "can_create_pub_sub",
		IsEnabled: func(flagKey string, r *http.Request, ff FeatureFlag) error {
			//limit pub sub to only groups
			if r.URL.Path == "/api/v1/sources" && r.Method == http.MethodPost {
				group := middleware.GetGroupFromContext(r.Context())

				isEnabled, err := ff.IsEnabled(flagKey, map[string]string{"group_id": group.UID})
				if err != nil {
					return err
				}

				if !isEnabled {
					return errors.New("pub sub is not available for you yet, sorry")
				}

				return nil
			}

			return nil
		},
	},
}

func CanAccessFeature(ff FeatureFlag) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, feature := range Features {
				err := feature.IsEnabled(feature.FlagKey, r, ff)
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

				continue
			}

			next.ServeHTTP(w, r)
		})
	}
}
