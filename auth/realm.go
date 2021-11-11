package auth

type Realm interface {
	GetName() string
	Authenticate(cred *Credential) (*AuthenticatedUser, error)
}

type RealmType string

const (
	RealmTypeAPIKey = RealmType("api_key_realm")
	RealmTypeBasic  = RealmType("basic_realm")
	//RealmTypeFile   = RealmType("file_realm")
	//RealmTypeVault = RealmType("vault_realm")
)

func (r RealmType) String() string {
	return string(r)
}

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
