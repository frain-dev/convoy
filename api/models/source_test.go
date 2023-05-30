package models

import (
	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCreateSource_Validate(t *testing.T) {
	type args struct {
		source *CreateSource
	}

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
