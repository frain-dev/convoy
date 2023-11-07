package newcloud

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
)

func (m *Migrator) RunEndpointMigration() error {
	for _, p := range m.projects {
		endpoints, err := m.loadProjectEndpoints(p.OrganisationID, p.UID, pagedResponse{})
		if err != nil {
			return err
		}

		err = m.SaveEndpoints(context.Background(), endpoints)
		if err != nil {
			return fmt.Errorf("failed to save endpoints: %v", err)
		}
		return nil
	}

	return nil
}

const (
	saveEndpoints = `
	INSERT INTO convoy.endpoints (
		id, title, status, secrets, owner_id, target_url, description, http_timeout,
		rate_limit, rate_limit_duration, advanced_signatures, slack_webhook_url,
		support_email, app_id, project_id, authentication_type, authentication_type_api_key_header_name,
		authentication_type_api_key_header_value, created_at, updated_at, deleted_at
	)
	VALUES
	  (
		:id, :title, :status, :secrets, :owner_id, :target_url, :description, :http_timeout,
		:rate_limit, :rate_limit_duration, :advanced_signatures, :slack_webhook_url,
		:support_email, :app_id, :project_id, :authentication_type, :authentication_type_api_key_header_name,
		:authentication_type_api_key_header_value, :created_at, :updated_at, :deleted_at
	  )
	`
)

func (m *Migrator) SaveEndpoints(ctx context.Context, endpoints []datastore.Endpoint) error {
	values := make([]map[string]interface{}, 0, len(endpoints))

	for _, endpoint := range endpoints {
		ac := endpoint.GetAuthConfig()

		values = append(values, map[string]interface{}{
			"id":                  endpoint.UID,
			"title":               endpoint.Title,
			"status":              endpoint.Status,
			"secrets":             endpoint.Secrets,
			"owner_id":            endpoint.OwnerID,
			"target_url":          endpoint.TargetURL,
			"description":         endpoint.Description,
			"http_timeout":        endpoint.HttpTimeout,
			"rate_limit":          endpoint.RateLimit,
			"rate_limit_duration": endpoint.RateLimitDuration,
			"advanced_signatures": endpoint.AdvancedSignatures,
			"slack_webhook_url":   endpoint.SlackWebhookURL,
			"support_email":       endpoint.SupportEmail,
			"app_id":              endpoint.AppID,
			"project_id":          endpoint.ProjectID,
			"authentication_type": ac.Type,
			"authentication_type_api_key_header_name":  ac.ApiKey.HeaderName,
			"authentication_type_api_key_header_value": ac.ApiKey.HeaderValue,
			"created_at": endpoint.CreatedAt,
			"updated_at": endpoint.UpdatedAt,
			"deleted_at": endpoint.DeletedAt,
		})
	}

	_, err := m.newDB.NamedExecContext(ctx, saveEndpoints, values)
	return err
}
