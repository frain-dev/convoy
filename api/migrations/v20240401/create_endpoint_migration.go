package v20240401

import "context"

// EndpointResponseMigration handles response migration for models.EndpointResponse
// This version changed field names: url→target_url, name→title
type EndpointResponseMigration struct{}

func (m *EndpointResponseMigration) MigrateForward(ctx context.Context, data any) (any, error) {
	// No forward migration needed for responses
	return data, nil
}

func (m *EndpointResponseMigration) MigrateBackward(ctx context.Context, data any) (any, error) {
	d, ok := data.(map[string]interface{})
	if !ok {
		return data, nil
	}

	// Rename url → target_url for old clients
	if url, ok := d["url"]; ok {
		d["target_url"] = url
		delete(d, "url")
	}

	// Rename name → title for old clients
	if name, ok := d["name"]; ok {
		d["title"] = name
		delete(d, "name")
	}

	return d, nil
}
