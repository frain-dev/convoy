package email

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/frain-dev/convoy/mocks"
)

func Test_Build(t *testing.T) {
	tests := []struct {
		name    string
		glob    string
		wantErr bool
		params  interface{}
		mockFn  func(c *mocks.MockSmtpClient)
	}{
		{
			name: "invalid template",
			glob: "rubbish",
			mockFn: func(c *mocks.MockSmtpClient) {
				c.EXPECT().SendEmail(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr: true,
		},
		{
			name: "invalid - missing params",
			glob: "endpoint.update.html",
			params: struct {
				URL     string
				LogoURL string
				Status  string
			}{
				URL:     "https://endpoint.com",
				LogoURL: "https://endpoint-logo-url.com",
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := buildClient(ctrl)

			// Act.
			e := NewEmail(client)
			err := e.Build(tc.glob, tc.params)

			// Assert.
			if tc.wantErr {
				require.Error(t, err)
			}
		})
	}
}

func Test_Build_FooterRendersCurrentYear(t *testing.T) {
	templates := []string{
		"user.verify.email.html",
		"reset.password.html",
		"organisation.invite.html",
		"endpoint.update.html",
	}

	params := map[string]string{
		"recipient_name":         "Jon",
		"email":                  "jon@example.com",
		"email_verification_url": "https://example.com/verify",
		"inviter_name":           "Jane",
		"invite_url":             "https://example.com/invite",
		"password_reset_url":     "https://example.com/reset",
		"expires_at":             "2026-01-01",
		"endpoint_status":        "inactive",
		"name":                   "test-endpoint",
		"target_url":             "https://example.com/endpoint",
		"response_body":          "",
		"failure_msg":            "connection refused",
	}

	for _, glob := range templates {
		t.Run(glob, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			e := NewEmail(buildClient(ctrl))
			require.NoError(t, e.Build(glob, params))

			body := e.body.String()
			require.Contains(t, body, fmt.Sprintf("© %d Frain Technologies", time.Now().Year()))
			require.NotContains(t, body, "© 2024")
		})
	}
}

func buildClient(ctrl *gomock.Controller) smtp.SmtpClient {
	return mocks.NewMockSmtpClient(ctrl)
}
