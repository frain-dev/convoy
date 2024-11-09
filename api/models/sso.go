package models

type SSORequest struct {
	LicenseKey string `json:"license_key"`
}

type SSOResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		RedirectURL string `json:"redirect_url"`
	} `json:"data"`
}

type SSOLoginResponse struct {
	RedirectURL string `json:"redirectUrl"`
}

type SSOTokenRequest struct {
	Token string `json:"token"`
}

type Payload struct {
	Email                  string `json:"email"`
	OrganizationID         string `json:"organizationId"`
	OrganizationExternalID string `json:"organizationExternalId"`
	SamlFlowID             string `json:"samlFlowId"`
}

type SSOTokenResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Payload Payload `json:"payload"`
	} `json:"data"`
}
