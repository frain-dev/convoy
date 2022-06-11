package notification

import "context"

type Notification struct {
	Text              string `json:"text,omitempty"`
	Email             string `json:"email,omitempty"`
	LogoURL           string `json:"logo_url,omitempty"`
	Subject           string `json:"subject,omitempty"`
	TargetURL         string `json:"target_url,omitempty"`
	EndpointStatus    string `json:"endpoint_status,omitempty"`
	InviteURL         string `json:"invite_url,omitempty"`
	EmailTemplateName string `json:"email_template_name,omitempty"`
	OrganisationName  string `json:"organisation_name,omitempty"`
	InviterName       string `json:"inviter_name,omitempty"`
	PasswordResetURL  string `json:"reset_url,omitempty"`
	RecipientName     string `json:"recipient_name,omitempty"`
	ExpiresAt         string `json:"expires_at,omitempty"`
}

type Sender interface {
	SendNotification(context.Context, *Notification) error
}
