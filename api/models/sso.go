package models

type SSOLoginResponse struct {
	RedirectURL string `json:"redirectUrl"`
}

type Payload struct {
	Email                  string `json:"email"`
	FirstName              string `json:"first_name"`
	LastName               string `json:"last_name"`
	OrganizationID         string `json:"organization_id"`
	OrganizationExternalID string `json:"organization_external_id"`
	ID                     string `json:"id"`
}

type SSOTokenResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Payload Payload `json:"payload"`
	} `json:"data"`
}
