package models

import (
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"gopkg.in/guregu/null.v4"
)

type PagedResponse struct {
	Content    interface{}               `json:"content,omitempty"`
	Pagination *datastore.PaginationData `json:"pagination,omitempty"`
}

type Organisation struct {
	Name         string `json:"name" bson:"name"`
	CustomDomain string `json:"custom_domain" bson:"custom_domain"`
}

type OrganisationInvite struct {
	InviteeEmail string    `json:"invitee_email" valid:"required~please provide a valid invitee email,email"`
	Role         auth.Role `json:"role" bson:"role"`
}

type APIKey struct {
	Name      string            `json:"name"`
	Role      Role              `json:"role"`
	Type      datastore.KeyType `json:"key_type"`
	ExpiresAt null.Time         `json:"expires_at"`
}

type PersonalAPIKey struct {
	Name       string `json:"name"`
	Expiration int    `json:"expiration"`
}

type Role struct {
	Type    auth.RoleType `json:"type"`
	Project string        `json:"project"`
	App     string        `json:"app,omitempty"`
}

type UpdateOrganisationMember struct {
	Role auth.Role `json:"role" bson:"role"`
}

type APIKeyByIDResponse struct {
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	Role      auth.Role         `json:"role"`
	Type      datastore.KeyType `json:"key_type"`
	ExpiresAt null.Time         `json:"expires_at,omitempty"`
	CreatedAt time.Time         `json:"created_at,omitempty"`
	UpdatedAt time.Time         `json:"updated_at,omitempty"`
}

type APIKeyResponse struct {
	APIKey
	Key       string    `json:"key"`
	UID       string    `json:"uid"`
	UserID    string    `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type PortalAPIKeyResponse struct {
	Key        string    `json:"key"`
	Role       auth.Role `json:"role"`
	Url        string    `json:"url,omitempty"`
	Type       string    `json:"key_type"`
	EndpointID string    `json:"endpoint_id,omitempty"`
	ProjectID  string    `json:"project_id,omitempty"`
}

type UserInviteTokenResponse struct {
	Token *datastore.OrganisationInvite `json:"token"`
	User  *datastore.User               `json:"user"`
}

type DeliveryAttempt struct {
	MessageID  string `json:"msg_id" bson:"msg_id"`
	APIVersion string `json:"api_version" bson:"api_version"`
	IPAddress  string `json:"ip" bson:"ip"`

	Status    string `json:"status" bson:"status"`
	CreatedAt int64  `json:"created_at" bson:"created_at"`

	MessageResponse MessageResponse `json:"response" bson:"response"`
}

type MessageResponse struct {
	Status int             `json:"status" bson:"status"`
	Data   json.RawMessage `json:"data" bson:"data"`
}
type ExpireSecret struct {
	// New Endpoint secret value.
	Secret string `json:"secret"`

	// Amount of time to wait before expiring the old endpoint secret.
	// If AdvancedSignatures is turned on for the project, signatures for both secrets will be generated up until
	// the old signature is expired.
	Expiration int `json:"expiration"`
}

type DashboardSummary struct {
	EventsSent   uint64                     `json:"events_sent" bson:"events_sent"`
	Applications int                        `json:"apps" bson:"apps"`
	Period       string                     `json:"period" bson:"period"`
	PeriodData   *[]datastore.EventInterval `json:"event_data,omitempty" bson:"event_data"`
}

type WebhookRequest struct {
	Event string          `json:"event" bson:"event"`
	Data  json.RawMessage `json:"data" bson:"data"`
}

type CreateEndpointApiKey struct {
	Project    *datastore.Project
	Endpoint   *datastore.Endpoint
	Name       string `json:"name"`
	BaseUrl    string
	KeyType    datastore.KeyType `json:"key_type"`
	Expiration int               `json:"expiration"`
}

type PortalLink struct {
	// Portal Link Name
	Name string `json:"name" valid:"required~please provide the name field"`

	// IDs of endpoints in this portal link
	Endpoints []string `json:"endpoints"`

	// Alternatively specify OwnerID, the portal link will inherit all the endpoints with this owner ID
	OwnerID string `json:"owner_id"`

	// Specify with endpoint management can be done through the Portal Link UI
	CanManageEndpoint bool `json:"can_manage_endpoint"`
}

type PortalLinkResponse struct {
	UID               string                     `json:"uid"`
	Name              string                     `json:"name"`
	ProjectID         string                     `json:"project_id"`
	OwnerID           string                     `json:"owner_id"`
	Endpoints         []string                   `json:"endpoints"`
	EndpointCount     int                        `json:"endpoint_count"`
	CanManageEndpoint bool                       `json:"can_manage_endpoint"`
	Token             string                     `json:"token"`
	EndpointsMetadata datastore.EndpointMetadata `json:"endpoints_metadata"`
	URL               string                     `json:"url"`
	CreatedAt         time.Time                  `json:"created_at,omitempty"`
	UpdatedAt         time.Time                  `json:"updated_at,omitempty"`
	DeletedAt         null.Time                  `json:"deleted_at,omitempty"`
}

// NewListResponse is generic function for looping over
// a slice of type M and returning a slice of type T
func NewListResponse[T, M any](items []M, fn func(item M) T) []T {
	results := make([]T, 0)

	for _, item := range items {
		results = append(results, fn(item))
	}

	return results
}
