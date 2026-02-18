package service

type RedirectURLRequest struct {
	CallbackURL string `json:"callback_url"`
	Host        string `json:"host,omitempty"`
	OrgID       string `json:"org_id,omitempty"`
}

type RedirectURLData struct {
	RedirectURL      string `json:"redirect_url"`
	AuthorizationURL string `json:"authorization_url"`
}

type RedirectURLResponse struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    RedirectURLData `json:"data"`
}

type TokenValidationRequest struct {
	Token string `json:"token"`
	OrgID string `json:"org_id,omitempty"`
}

type TokenValidationData struct {
	Payload UserProfile `json:"payload"`
}

type TokenValidationResponse struct {
	Status  bool                `json:"status"`
	Message string              `json:"message"`
	Data    TokenValidationData `json:"data"`
}

type UserProfile struct {
	Email                  string `json:"email"`
	FirstName              string `json:"first_name"`
	LastName               string `json:"last_name"`
	OrganizationID         string `json:"organization_id"`
	OrganizationExternalID string `json:"organization_external_id"`
	ID                     string `json:"id"`
	ProfileID              string `json:"profile_id"`
}

// AdminPortalRequest is the body sent to Overwatch POST /sso/admin-portal.
type AdminPortalRequest struct {
	ReturnURL  string `json:"return_url"`
	SuccessURL string `json:"success_url"`
	OrgID      string `json:"org_id,omitempty"`
}

// AdminPortalData is the data object in the admin-portal response.
type AdminPortalData struct {
	PortalURL string `json:"portal_url"`
	ExpiresIn int    `json:"expires_in"`
}

// AdminPortalResponse is the response from Overwatch POST /sso/admin-portal.
type AdminPortalResponse struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    AdminPortalData `json:"data"`
}
