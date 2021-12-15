package config

import "github.com/frain-dev/convoy/auth"

type BasicAuth struct {
	Username string    `json:"username"`
	Password string    `json:"password"`
	Role     auth.Role `json:"role"`
}

type APIKeyAuth struct {
	APIKey string    `json:"api_key"`
	Role   auth.Role `json:"role"`
}
