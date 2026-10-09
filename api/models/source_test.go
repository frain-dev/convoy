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

func TestUpdateSource_ValidateRejectsPayloadSignatureMetadataEventTypeLocation(t *testing.T) {
	name := "Convoy-Prod"
	location := "request.header.X-Gitlab-Event"
	source := &UpdateSource{
		Name:              &name,
		Type:              datastore.HTTPSource,
		EventTypeLocation: &location,
		Verifier: VerifierConfig{
			Type: datastore.HMacVerifier,
			HMac: &HMac{
				Encoding: datastore.Base64Encoding,
				Header:   "X-Convoy-Signature",
				Hash:     "SHA512",
				Secret:   "Convoy-Secret",
			},
		},
	}

	require.Error(t, source.Validate())
}

func TestUpdateSource_Validate(t *testing.T) {
	name := "Convoy-Prod"
	tests := []struct {
		name        string
		source      *UpdateSource
		wantErr     bool
		expectedErr string
	}{
		{
			name: "should_pass_validation_with_valid_provider_and_hmac",
			source: &UpdateSource{
				Name:     &name,
				Type:     datastore.HTTPSource,
				Provider: datastore.GithubSourceProvider,
				Verifier: VerifierConfig{
					Type: datastore.HMacVerifier,
					HMac: &HMac{
						Encoding: datastore.Base64Encoding,
						Header:   "X-Hub-Signature-256",
						Hash:     "SHA256",
						Secret:   "Convoy-Secret",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_pass_validation_without_provider",
			source: &UpdateSource{
				Name: &name,
				Type: datastore.HTTPSource,
				Verifier: VerifierConfig{
					Type: datastore.NoopVerifier,
				},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_github_provider_with_empty_verifier",
			source: &UpdateSource{
				Name:     &name,
				Type:     datastore.HTTPSource,
				Provider: datastore.GithubSourceProvider,
				Verifier: VerifierConfig{},
			},
			wantErr:     true,
			expectedErr: "hmac secret is required for github source",
		},
		{
			name: "should_error_for_shopify_provider_with_empty_verifier",
			source: &UpdateSource{
				Name:     &name,
				Type:     datastore.HTTPSource,
				Provider: datastore.ShopifySourceProvider,
				Verifier: VerifierConfig{},
			},
			wantErr:     true,
			expectedErr: "hmac secret is required for shopify source",
		},
		{
			name: "should_error_for_twitter_provider_with_empty_verifier",
			source: &UpdateSource{
				Name:     &name,
				Type:     datastore.HTTPSource,
				Provider: datastore.TwitterSourceProvider,
				Verifier: VerifierConfig{},
			},
			wantErr:     true,
			expectedErr: "hmac secret is required for twitter source",
		},
		{
			name: "should_error_for_provider_with_nil_hmac",
			source: &UpdateSource{
				Name:     &name,
				Type:     datastore.HTTPSource,
				Provider: datastore.GithubSourceProvider,
				Verifier: VerifierConfig{
					Type: datastore.HMacVerifier,
					HMac: nil,
				},
			},
			wantErr:     true,
			expectedErr: "hmac secret is required for github source",
		},
		{
			name: "should_error_for_provider_with_empty_hmac_secret",
			source: &UpdateSource{
				Name:     &name,
				Type:     datastore.HTTPSource,
				Provider: datastore.GithubSourceProvider,
				Verifier: VerifierConfig{
					Type: datastore.HMacVerifier,
					HMac: &HMac{
						Secret: "",
					},
				},
			},
			wantErr:     true,
			expectedErr: "hmac secret is required for github source",
		},
		{
			name: "should_error_for_provider_with_non_hmac_verifier",
			source: &UpdateSource{
				Name:     &name,
				Type:     datastore.HTTPSource,
				Provider: datastore.GithubSourceProvider,
				Verifier: VerifierConfig{
					Type: datastore.APIKeyVerifier,
					ApiKey: &ApiKey{
						HeaderName:  "X-API-Key",
						HeaderValue: "secret-value",
					},
				},
			},
			wantErr:     true,
			expectedErr: "hmac secret is required for github source",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.source.Validate()
			if tc.wantErr {
				require.Error(t, err)
				if tc.expectedErr != "" {
					require.Contains(t, err.Error(), tc.expectedErr)
				}
				return
			}

			require.NoError(t, err)
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
