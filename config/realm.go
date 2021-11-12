package config

type RealmOption struct {
	Type   string       `json:"type"`
	Path   string       `json:"path"`
	Url    string       `json:"url"`
	Basic  []BasicAuth  `json:"basic"`
	ApiKey []APIKeyAuth `json:"api_key"`
}

type BasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     Role   `json:"role"`
}

type APIKeyAuth struct {
	APIKey string `json:"api_key"`
	Role   Role   `json:"role"`
}
