package v20240401

import "context"

type EndpointResponseMigration struct{}

func (m *EndpointResponseMigration) MigrateForward(ctx context.Context, data any) (any, error) {
	return data, nil
}

func (m *EndpointResponseMigration) MigrateBackward(ctx context.Context, data any) (any, error) {
	d, ok := data.(map[string]interface{})
	if !ok {
		return data, nil
	}

	if url, ok := d["url"]; ok {
		d["target_url"] = url
		delete(d, "url")
	}

	if name, ok := d["name"]; ok {
		d["title"] = name
		delete(d, "name")
	}

	return d, nil
}
