package convoy

import "net/http"

type AddHeadersPlugin struct {
	config map[string]string
}

func (a *AddHeadersPlugin) Apply(w http.ResponseWriter, r *http.Request) error {

	for k, v := range a.config {
		w.Header().Add(k, v)
	}

	return nil
}

func (a *AddHeadersPlugin) Name() string { return "Add Headers" }

func (a *AddHeadersPlugin) IsEnabled() bool { return len(a.config) > 0 }
