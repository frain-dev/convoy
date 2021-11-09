package auth

type Realm interface {
	GetName() string
	Authenticate(cred *Credential) (*AuthenticatedUser, error)
}

type RealmType string

const (
	RealmTypeFile = RealmType("file_realm")
	//RealmTypeVault = RealmType("vault_realm")
)

type RealmOption struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Path   string `json:"path"`
	ApiKey string `json:"api_key"`
	Url    string `json:"url"`
}
