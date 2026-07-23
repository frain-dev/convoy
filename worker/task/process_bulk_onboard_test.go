package task

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestValidateOnboardItemURL(t *testing.T) {
	projectWith := func(enforceSecure bool) *datastore.Project {
		return &datastore.Project{
			Config: &datastore.ProjectConfig{
				SSL: &datastore.SSLConfiguration{EnforceSecureEndpoints: enforceSecure},
			},
		}
	}

	tests := map[string]struct {
		rawURL  string
		project *datastore.Project
		wantErr bool
	}{
		"https_url_is_valid": {
			rawURL:  "https://example.com/webhook",
			project: projectWith(true),
			wantErr: false,
		},
		"http_url_allowed_when_not_enforced": {
			rawURL:  "http://example.com/webhook",
			project: projectWith(false),
			wantErr: false,
		},
		"http_url_rejected_when_secure_enforced": {
			rawURL:  "http://example.com/webhook",
			project: projectWith(true),
			wantErr: true,
		},
		"empty_url_rejected": {
			rawURL:  "",
			project: projectWith(false),
			wantErr: true,
		},
		"invalid_scheme_rejected": {
			rawURL:  "ftp://example.com/webhook",
			project: projectWith(false),
			wantErr: true,
		},
		"garbage_url_rejected": {
			rawURL:  "://not-a-url",
			project: projectWith(false),
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateOnboardItemURL(tc.rawURL, tc.project)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
