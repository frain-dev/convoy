package models

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestCreateSource_Validate(t *testing.T) {
	tests := []struct {
		name    string
		source  *CreateSource
		wantErr bool
	}{
		{
			name: "should_pass_validation",
			source: &CreateSource{
				Name: "Convoy-Prod",
				Type: datastore.HTTPSource,
				CustomResponse: CustomResponse{
					Body:        "[accepted]",
					ContentType: "application/json",
				},
				Verifier: VerifierConfig{
					Type: datastore.HMacVerifier,
					HMac: &HMac{
						Encoding: datastore.Base64Encoding,
						Header:   "X-Convoy-Header",
						Hash:     "SHA512",
						Secret:   "Convoy-Secret",
					},
				},
			},
		},

		{
			name: "should_error_for_empty_name",
			source: &CreateSource{
				Name:     "",
				Type:     datastore.HTTPSource,
				Provider: datastore.GithubSourceProvider,
				Verifier: VerifierConfig{
					HMac: &HMac{
						Secret: "Convoy-Secret",
					},
				},
			},
			wantErr: true,
		},

		{
			name: "should_error_for_invalid_type",
			source: &CreateSource{
				Name:     "Convoy-prod",
				Type:     "abc",
				Provider: datastore.GithubSourceProvider,
				Verifier: VerifierConfig{
					HMac: &HMac{
						Secret: "Convoy-Secret",
					},
				},
			},
			wantErr: true,
		},

		{
			name: "should_error_for_empty_hmac_secret",
			source: &CreateSource{
				Name:     "Convoy-Prod",
				Type:     datastore.HTTPSource,
				Provider: datastore.GithubSourceProvider,
				Verifier: VerifierConfig{
					HMac: &HMac{
						Secret: "",
					},
				},
			},
			wantErr: true,
		},

		{
			name: "should_error_for_nil_hmac",
			source: &CreateSource{
				Name:     "Convoy-Prod",
				Type:     datastore.HTTPSource,
				Provider: datastore.GithubSourceProvider,
				Verifier: VerifierConfig{HMac: nil},
			},
			wantErr: true,
		},

		{
			name: "should_fail_invalid_source_configuration",
			source: &CreateSource{
				Name: "Convoy-Prod",
				Type: datastore.HTTPSource,
				Verifier: VerifierConfig{
					Type: datastore.HMacVerifier,
				},
			},
			wantErr: true,
		},
		{
			name: "should_error_for_hmac_source_with_header_event_type_location",
			source: &CreateSource{
				Name:              "Convoy-Prod",
				Type:              datastore.HTTPSource,
				EventTypeLocation: "request.header.X-Gitlab-Event",
				Verifier: VerifierConfig{
					Type: datastore.HMacVerifier,
					HMac: &HMac{
						Encoding: datastore.Base64Encoding,
						Header:   "X-Convoy-Signature",
						Hash:     "SHA512",
						Secret:   "Convoy-Secret",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "should_error_for_provider_source_with_query_event_type_location",
			source: &CreateSource{
				Name:              "Convoy-Prod",
				Type:              datastore.HTTPSource,
				Provider:          datastore.GithubSourceProvider,
				EventTypeLocation: "request.query.event_type",
				Verifier: VerifierConfig{
					HMac: &HMac{
						Secret: "Convoy-Secret",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			err := tc.source.Validate()
			if tc.wantErr {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestValidateEventTypeLocation(t *testing.T) {
	tests := []struct {
		name     string
		location string
		wantErr  bool
	}{
		{
			name:     "empty location",
			location: "",
		},
		{
			name:     "body location",
			location: "request.body.object_kind",
		},
		{
			name:     "nested body location",
			location: "request.body.project.path_with_namespace",
		},
		{
			name:     "header location",
			location: "request.header.X-Gitlab-Event",
		},
		{
			name:     "query location",
			location: "request.query.event_type",
		},
		{
			name:     "queryparam location",
			location: "request.queryparam.event_type",
		},
		{
			name:     "nested header selector",
			location: "request.header.x.event.type",
			wantErr:  true,
		},
		{
			name:     "nested query selector",
			location: "request.query.event.type",
			wantErr:  true,
		},
		{
			name:     "req queryparam location",
			location: "req.QueryParam.event_type",
		},
		{
			name:     "short location",
			location: "request.body",
			wantErr:  true,
		},
		{
			name:     "empty body selector",
			location: "request.body.",
			wantErr:  true,
		},
		{
			name:     "empty header selector",
			location: "request.header.",
			wantErr:  true,
		},
		{
			name:     "empty query selector",
			location: "request.query.",
			wantErr:  true,
		},
		{
			name:     "root with whitespace",
			location: " request.body.object_kind",
			wantErr:  true,
		},
		{
			name:     "scope with whitespace",
			location: "request. body.object_kind",
			wantErr:  true,
		},
		{
			name:     "selector with whitespace",
			location: "request.body. object_kind",
			wantErr:  true,
		},
		{
			name:     "unsupported root",
			location: "payload.body.object_kind",
			wantErr:  true,
		},
		{
			name:     "unsupported source",
			location: "request.cookie.event_type",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEventTypeLocation(tc.location)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}
