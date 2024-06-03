package email

import (
	"testing"

	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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

func buildClient(ctrl *gomock.Controller) smtp.SmtpClient {
	return mocks.NewMockSmtpClient(ctrl)
}
