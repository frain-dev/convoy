package v20251124

import (
	"encoding/json"
	"net/http"

	"github.com/frain-dev/convoy/internal/portal_links/models"
)

type UpdatePortalLinkRequestMigration struct{}

func NewUpdatePortalLinkRequestMigration() *UpdatePortalLinkRequestMigration {
	return &UpdatePortalLinkRequestMigration{}
}

func (c *UpdatePortalLinkRequestMigration) Migrate(b []byte, h http.Header) ([]byte, http.Header, error) {
	var payload models.UpdatePortalLinkRequest
	err := json.Unmarshal(b, &payload)
	if err != nil {
		return nil, nil, err
	}

	b, err = json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}

	return b, h, nil
}
