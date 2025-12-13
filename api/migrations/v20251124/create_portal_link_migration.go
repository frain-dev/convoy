package v20251124

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/internal/portal_links/models"
	"github.com/frain-dev/convoy/util"
)

type CreatePortalLinkRequestMigration struct{}

func NewCreatePortalLinkRequestMigration() *CreatePortalLinkRequestMigration {
	return &CreatePortalLinkRequestMigration{}
}

func (c *CreatePortalLinkRequestMigration) Migrate(b []byte, h http.Header) ([]byte, http.Header, error) {
	var payload models.CreatePortalLinkRequest
	err := json.Unmarshal(b, &payload)
	if err != nil {
		return nil, nil, err
	}

	// Validate owner_id is provided
	if util.IsStringEmpty(payload.OwnerID) {
		return nil, nil, errors.New("owner_id is required")
	}

	b, err = json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}

	return b, h, nil
}
