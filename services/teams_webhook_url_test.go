package services

import (
	"context"
	"strings"
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

// loadTeamsTestConfig mirrors the setup the endpoint service tests share: the URL
// validator reads config, and the ping probe would otherwise dial the test hosts.
func loadTeamsTestConfig(t *testing.T) {
	t.Helper()

	t.Setenv("CONVOY_DISPATCHER_SKIP_PING_VALIDATION", "true")

	require.NoError(t, config.LoadCaCert("", ""))
	require.NoError(t, config.LoadConfig("", func(c *config.Configuration) error {
		return envconfig.Process("convoy", c)
	}))
}

// teamsWebhookURL mirrors a Power Automate Workflows webhook. The sig query
// parameter is bearer-equivalent: possession of the URL is authorisation to post
// into the customer's channel.
const teamsWebhookURL = "https://example.logic.azure.com:443/workflows/abc/triggers/manual/paths/invoke?sig=notasecretbutpretend"

// privateSlackWebhookURL fails the outbound URL guard; Slack webhook URLs carry
// the secret in the path and must not be echoed on rejection.
const privateSlackWebhookURL = "http://127.0.0.1/hooks/slack/services/T/B/secret-token"

// privateTeamsWebhookURL fails the outbound URL guard, carrying a sig the
// rejection must not echo.
const privateTeamsWebhookURL = "http://127.0.0.1/workflows/abc/triggers/manual/paths/invoke?sig=notasecretbutpretend"

func TestUpdateEndpointService_TeamsWebhookURL(t *testing.T) {
	loadTeamsTestConfig(t)

	project := &datastore.Project{UID: "1234567890", Config: &datastore.DefaultProjectConfig}

	tests := []struct {
		name string
		// licensed toggles AdvancedEndpointMgmt, the same gate slack_webhook_url
		// uses. The send path only checks the column is non-empty, so the write
		// gate is the only thing keeping an unlicensed instance from alerting.
		licensed   bool
		existing   string
		update     *string
		wantStored string
		wantErrMsg string
	}{
		{
			name:       "licensed set",
			licensed:   true,
			update:     stringPtr(teamsWebhookURL),
			wantStored: teamsWebhookURL,
		},
		{
			name:       "licensed clear",
			licensed:   true,
			existing:   teamsWebhookURL,
			update:     stringPtr(""),
			wantStored: "",
		},
		{
			name:       "unlicensed set is dropped",
			licensed:   false,
			update:     stringPtr(teamsWebhookURL),
			wantStored: "",
		},
		{
			// An unlicensed update must not silently wipe a value written while
			// the instance was licensed, matching slack_webhook_url.
			name:       "unlicensed update leaves existing value",
			licensed:   false,
			existing:   teamsWebhookURL,
			update:     stringPtr("https://other.example.com/hook"),
			wantStored: teamsWebhookURL,
		},
		{
			name:       "omitted field leaves existing value",
			licensed:   true,
			existing:   teamsWebhookURL,
			update:     nil,
			wantStored: teamsWebhookURL,
		},
		{
			name:       "private address rejected",
			licensed:   true,
			update:     stringPtr(privateTeamsWebhookURL),
			wantErrMsg: "invalid teams webhook url",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			stored := &datastore.Endpoint{UID: "endpoint1", TeamsWebhookURL: tc.existing}

			as := provideUpdateEndpointService(ctrl, models.UpdateEndpoint{
				Name:            stringPtr("Endpoint1"),
				URL:             "https://www.google.com/webhp",
				HttpTimeout:     20,
				TeamsWebhookURL: tc.update,
			}, stored, project)

			repo, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
			repo.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), project.UID).Times(1).Return(stored, nil)

			licenser, _ := as.Licenser.(*mocks.MockLicenser)
			licenser.EXPECT().IpRules().AnyTimes().Return(true)
			licenser.EXPECT().CustomCertificateAuthority().AnyTimes().Return(true)
			licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(tc.licensed)

			if tc.wantErrMsg == "" {
				repo.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Cond(func(x any) bool {
					return x.(*datastore.Endpoint).TeamsWebhookURL == tc.wantStored
				}), gomock.Any()).Times(1).Return(nil)
			}

			endpoint, err := as.Run(context.Background())

			if tc.wantErrMsg != "" {
				require.Error(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantStored, endpoint.TeamsWebhookURL)
		})
	}
}

// TestTeamsWebhookURLRejectionDoesNotLeakSignature covers the whole error chain,
// not just Error(). util.ValidateOutboundURL wraps url.Parse, whose errors quote
// the URL they failed on, so a wrapped error would carry the sig into any handler
// or log line that unwraps it.
func TestTeamsWebhookURLRejectionDoesNotLeakSignature(t *testing.T) {
	loadTeamsTestConfig(t)

	project := &datastore.Project{UID: "1234567890", Type: datastore.OutgoingProject, Config: &datastore.DefaultProjectConfig}

	assertNoSignature := func(t *testing.T, err error) {
		t.Helper()

		require.Error(t, err)

		svcErr, ok := err.(*ServiceError)
		require.True(t, ok)
		require.Equal(t, "invalid teams webhook url", svcErr.Error())

		// Unwrap must not hand a caller the URL either.
		require.NoError(t, svcErr.Unwrap())

		for _, s := range []string{svcErr.Error(), svcErr.ErrMsg} {
			require.False(t, strings.Contains(s, "sig="), "error text leaked the signature: %s", s)
			require.False(t, strings.Contains(s, "127.0.0.1"), "error text leaked the host: %s", s)
		}
	}

	t.Run("create", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		as := provideCreateEndpointService(ctrl, models.CreateEndpoint{
			Name:            "endpoint",
			Secret:          "1234",
			URL:             "https://google.com",
			TeamsWebhookURL: privateTeamsWebhookURL,
		}, project.UID)

		projectRepo, _ := as.ProjectRepo.(*mocks.MockProjectRepository)
		projectRepo.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Times(1).Return(project, nil)

		licenser, _ := as.Licenser.(*mocks.MockLicenser)
		licenser.EXPECT().IpRules().AnyTimes().Return(true)
		licenser.EXPECT().CustomCertificateAuthority().AnyTimes().Return(true)
		licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(true)

		_, err := as.Run(context.Background())
		assertNoSignature(t, err)
	})

	t.Run("update", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		stored := &datastore.Endpoint{UID: "endpoint1"}

		as := provideUpdateEndpointService(ctrl, models.UpdateEndpoint{
			Name:            stringPtr("Endpoint1"),
			URL:             "https://www.google.com/webhp",
			HttpTimeout:     20,
			TeamsWebhookURL: stringPtr(privateTeamsWebhookURL),
		}, stored, project)

		repo, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
		repo.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), project.UID).Times(1).Return(stored, nil)

		licenser, _ := as.Licenser.(*mocks.MockLicenser)
		licenser.EXPECT().IpRules().AnyTimes().Return(true)
		licenser.EXPECT().CustomCertificateAuthority().AnyTimes().Return(true)
		licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(true)

		_, err := as.Run(context.Background())
		assertNoSignature(t, err)
	})

	t.Run("create slack", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		as := provideCreateEndpointService(ctrl, models.CreateEndpoint{
			Name:            "endpoint",
			Secret:          "1234",
			URL:             "https://google.com",
			SlackWebhookURL: privateSlackWebhookURL,
		}, project.UID)

		projectRepo, _ := as.ProjectRepo.(*mocks.MockProjectRepository)
		projectRepo.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Times(1).Return(project, nil)

		licenser, _ := as.Licenser.(*mocks.MockLicenser)
		licenser.EXPECT().IpRules().AnyTimes().Return(true)
		licenser.EXPECT().CustomCertificateAuthority().AnyTimes().Return(true)
		licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(true)

		_, err := as.Run(context.Background())
		assertNoSlackSecret(t, err)
	})

	t.Run("update slack", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		stored := &datastore.Endpoint{UID: "endpoint1"}

		as := provideUpdateEndpointService(ctrl, models.UpdateEndpoint{
			Name:            stringPtr("Endpoint1"),
			URL:             "https://www.google.com/webhp",
			HttpTimeout:     20,
			SlackWebhookURL: stringPtr(privateSlackWebhookURL),
		}, stored, project)

		repo, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
		repo.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), project.UID).Times(1).Return(stored, nil)

		licenser, _ := as.Licenser.(*mocks.MockLicenser)
		licenser.EXPECT().IpRules().AnyTimes().Return(true)
		licenser.EXPECT().CustomCertificateAuthority().AnyTimes().Return(true)
		licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(true)

		_, err := as.Run(context.Background())
		assertNoSlackSecret(t, err)
	})
}

func assertNoSlackSecret(t *testing.T, err error) {
	t.Helper()

	require.Error(t, err)

	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	require.Equal(t, "invalid slack webhook url", svcErr.Error())
	require.NoError(t, svcErr.Unwrap())

	for _, s := range []string{svcErr.Error(), svcErr.ErrMsg} {
		require.False(t, strings.Contains(s, "127.0.0.1"), "error text leaked the host: %s", s)
		require.False(t, strings.Contains(s, "secret-token"), "error text leaked the webhook secret: %s", s)
	}
}
