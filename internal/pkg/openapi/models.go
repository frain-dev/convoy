package openapi

// WebhookSchema represents a webhook schema extracted from OpenAPI spec
type WebhookSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Schema      map[string]interface{} `json:"schema"`
}

// WebhookCollection represents a collection of webhook schemas
type WebhookCollection struct {
	Webhooks []WebhookSchema `json:"webhooks"`
}
