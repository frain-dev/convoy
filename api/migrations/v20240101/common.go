package v20240101

import (
	"fmt"
	"time"

	"github.com/fatih/structs"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"gopkg.in/guregu/null.v4"
)

type direction string

const (
	forward  direction = "forward"
	backward direction = "backward"
)

type endpointResponse struct {
	*oldEndpoint
}

type oldCreateEndpoint struct {
	URL                string                         `json:"url" valid:"required~please provide a url for your endpoint"`
	Secret             string                         `json:"secret"`
	OwnerID            string                         `json:"owner_id"`
	Description        string                         `json:"description"`
	Name               string                         `json:"name" valid:"required~please provide your endpointName"`
	SupportEmail       string                         `json:"support_email" valid:"email~please provide a valid email"`
	IsDisabled         bool                           `json:"is_disabled"`
	SlackWebhookURL    string                         `json:"slack_webhook_url"`
	HttpTimeout        string                         `json:"http_timeout"`
	RateLimit          int                            `json:"rate_limit"`
	AdvancedSignatures *bool                          `json:"advanced_signatures"`
	RateLimitDuration  string                         `json:"rate_limit_duration"`
	Authentication     *models.EndpointAuthentication `json:"authentication"`
	AppID              string
}

type oldEndpoint struct {
	UID                string            `json:"uid" db:"id"`
	ProjectID          string            `json:"project_id" db:"project_id"`
	OwnerID            string            `json:"owner_id,omitempty" db:"owner_id"`
	TargetURL          string            `json:"target_url" db:"target_url"`
	Title              string            `json:"title" db:"title"`
	Secrets            datastore.Secrets `json:"secrets" db:"secrets"`
	AdvancedSignatures bool              `json:"advanced_signatures" db:"advanced_signatures"`
	Description        string            `json:"description" db:"description"`
	SlackWebhookURL    string            `json:"slack_webhook_url,omitempty" db:"slack_webhook_url"`
	SupportEmail       string            `json:"support_email,omitempty" db:"support_email"`
	AppID              string            `json:"-" db:"app_id"` // Deprecated but necessary for backward compatibility

	HttpTimeout string                   `json:"http_timeout" db:"http_timeout"`
	RateLimit   int                      `json:"rate_limit" db:"rate_limit"`
	Events      int64                    `json:"events,omitempty" db:"event_count"`
	Status      datastore.EndpointStatus `json:"status" db:"status"`

	RateLimitDuration string                                 `json:"rate_limit_duration" db:"rate_limit_duration"`
	Authentication    *datastore.EndpointAuthentication      `json:"authentication" db:"authentication"`
	CircuitBreaker    *datastore.CircuitBreakerConfiguration `json:"circuit_breaker"`

	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt null.Time `json:"deleted_at,omitempty" db:"deleted_at" swaggertype:"string"`
}

func transformDurationStringToInt(d string) (uint64, error) {
	id, err := time.ParseDuration(d)
	if err != nil {
		return 0, err
	}

	return uint64(id.Seconds()), nil
}

func transformIntToDurationString(t uint64) (string, error) {
	td := time.Duration(t) * time.Second
	return td.String(), nil
}

func migrateEndpoint(oldPayload, newPayload interface{}, direction direction) error {
	oldStruct := structs.New(oldPayload)
	newStruct := structs.New(newPayload)

	var err error
	for _, f := range oldStruct.Fields() {
		if f.IsZero() {
			continue
		}

		value := f.Value()
		jsonTag := f.Tag("json")
		if jsonTag == "http_timeout" || jsonTag == "rate_limit_duration" {
			switch direction {
			case forward:
				newValue, ok := f.Value().(string)
				if !ok {
					return fmt.Errorf("invalid type for %s field", jsonTag)
				}

				value, err = transformDurationStringToInt(newValue)
				if err != nil {
					return err
				}
			case backward:
				newValue, ok := f.Value().(uint64)
				if !ok {
					return fmt.Errorf("invalid type for %s field", jsonTag)
				}

				value, err = transformIntToDurationString(newValue)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("invalid direction %s", direction)
			}
		}

		err = newStruct.Field(f.Name()).Set(value)
		if err != nil {
			return err
		}
	}

	return nil
}
