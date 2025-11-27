package v20251124

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/api/migrations"
	"github.com/frain-dev/convoy/api/models"
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

	// Check if migration needs to update endpoint owner_ids: version is < 2025-11-24 and endpoints were provided
	version := h.Get("X-Convoy-Version")
	needsEndpointUpdate := version < "2025-11-24" && len(payload.Endpoints) > 0

	// Signal that business logic needs to update endpoint owner_ids
	if needsEndpointUpdate {
		h.Set(migrations.UpdateEndpointOwnerIDHeader, "true")
	}

	b, err = json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}

	return b, h, nil
}
