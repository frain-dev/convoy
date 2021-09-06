package convoy

import "net/http"

type Plugin interface {
	Apply(http.ResponseWriter, *http.Request) error
	Name() string
	IsEnabled() bool
}
