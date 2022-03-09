package config

import (
	"encoding/json"

	"github.com/frain-dev/convoy/auth"
)

type BasicAuthConfig []BasicAuth

type BasicAuth struct {
	Username string    `json:"username"`
	Password string    `json:"password"`
	Role     auth.Role `json:"role"`
}

// Decode loads in config from an env var named `CONVOY_BASIC_AUTH_CONFIG`
func (b *BasicAuthConfig) Decode(value string) error {
	config := BasicAuthConfig{}
	err := json.Unmarshal([]byte(value), &config)

	*b = config
	return err
}

type APIKeyAuthConfig []APIKeyAuth

type APIKeyAuth struct {
	APIKey string    `json:"api_key"`
	Role   auth.Role `json:"role"`
}

// Decode loads in config from an env var named `CONVOY_API_KEY_CONFIG`
func (a *APIKeyAuthConfig) Decode(value string) error {
	config := APIKeyAuthConfig{}
	err := json.Unmarshal([]byte(value), &config)

	*a = config
	return err
}
