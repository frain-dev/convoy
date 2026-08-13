package email

import (
	"fmt"
	"regexp"
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

var emailTemplates = []string{
	"user.verify.email.html",
	"reset.password.html",
	"organisation.invite.html",
	"endpoint.update.html",
	"twitter.source.html",
}

func emailParams() map[string]string {
	return map[string]string{
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
		"source_name":            "twitter-source",
		"crc_verified_at":        "2026-01-01",
	}
}

func Test_Build_FooterRendersCurrentYear(t *testing.T) {
	for _, glob := range emailTemplates {
		t.Run(glob, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			e := NewEmail(buildClient(ctrl))
			require.NoError(t, e.Build(glob, emailParams()))

			body := e.body.String()
			require.Contains(t, body, fmt.Sprintf("© %d Frain Technologies", time.Now().Year()))
			require.NotContains(t, body, "© 2024")
			require.Contains(t, body, "#0082F9")
			require.Contains(t, body, "#F7F7F7")
			require.Contains(t, body, "https://www.getconvoy.io/images/email/email-logo-white.png")
			require.Contains(t, body, "https://www.getconvoy.io/images/email/email-dots-left.png")
			require.Contains(t, body, "https://www.getconvoy.io/images/email/email-dots-right.png")
		})
	}
}

var msoConditional = regexp.MustCompile(`(?s)<!--\[if [^\]]*]>.*?<!\[endif]-->`)

// Gmail's Android app drops max-width on tables and honours the HTML width
// attribute instead, so a fixed pixel width there overflows the viewport and
// the message gets clipped. Everything outside the MSO conditionals has to stay
// fluid; Outlook keeps a fixed width through the ghost table inside them.
func Test_Build_LayoutIsFluid(t *testing.T) {
	for _, glob := range emailTemplates {
		t.Run(glob, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			e := NewEmail(buildClient(ctrl))
			require.NoError(t, e.Build(glob, emailParams()))

			body := e.body.String()
			fluid := msoConditional.ReplaceAllString(body, "")
			require.NotContains(t, fluid, `width="656"`)
			require.NotContains(t, fluid, `width="582"`)
			require.NotContains(t, fluid, `style="width: 656px`)
			require.NotContains(t, fluid, `style="width: 582px`)
			require.Contains(t, fluid, "max-width: 656px")
			require.Contains(t, fluid, "@media only screen and (max-width: 600px)")
			require.Contains(t, fluid, `class="gutter"`)
			require.Contains(t, fluid, `class="card-pad"`)

			// Outlook ignores max-width, so it needs the ghost table to hold the
			// 656px column its VML header band is cut for.
			require.Contains(t, body, `<table role="presentation" width="656"`)
		})
	}
}

func buildClient(ctrl *gomock.Controller) smtp.SmtpClient {
	return mocks.NewMockSmtpClient(ctrl)
}
