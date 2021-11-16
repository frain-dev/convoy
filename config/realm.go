package config

type BasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     Role   `json:"role"`
}

type APIKeyAuth struct {
	APIKey string `json:"api_key"`
	Role   Role   `json:"role"`
}
