package api_key

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
)

func TestFileRealm_Authenticate(t *testing.T) {
	apiKeyAuth := []auth.APIKeyAuth{
		{
			APIKey: "avcbajbwrohw@##Q39uekvsmbvxc.fdjhd",
			Role: auth.Role{
				Type:  auth.RoleUIAdmin,
				Group: "sendcash-pay",
			},
		},
	}

	ar, err := NewAPIKeyRealm(apiKeyAuth)
	if err != nil {
		require.Nil(t, err)
		return
	}

	type args struct {
		cred *auth.Credential
	}
	tests := []struct {
		name       string
		args       args
		want       *auth.AuthenticatedUser
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_authenticate_apiKey_cred_successfully",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "avcbajbwrohw@##Q39uekvsmbvxc.fdjhd",
				},
			},
			want: &auth.AuthenticatedUser{
				AuthenticatedByRealm: "api_key_realm",
				Credential: auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "avcbajbwrohw@##Q39uekvsmbvxc.fdjhd",
				},
				Role: auth.Role{
					Type:  auth.RoleUIAdmin,
					Group: "sendcash-pay",
				},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_wrong_apiKey",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "1234",
				},
			},
			want:       nil,
			wantErr:    true,
			wantErrMsg: "credential not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ar.Authenticate(tt.args.cred)

			if tt.wantErr {
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, got, tt.want)
		})
	}
}

func TestNewAPIKeyRealm(t *testing.T) {
	type args struct {
		apiKeyAuth []auth.APIKeyAuth
	}
	tests := []struct {
		name       string
		args       args
		want       *APIKeyRealm
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_initialize_file_realm_successfully",
			args: args{
				apiKeyAuth: []auth.APIKeyAuth{
					{
						APIKey: "avcbajbwrohw@##Q39uekvsmbvxc.fdjhd",
						Role: auth.Role{
							Type:  auth.RoleUIAdmin,
							Group: "sendcash-pay",
						},
					},
				},
			},
			want: &APIKeyRealm{
				APIKey: []auth.APIKeyAuth{
					{
						APIKey: "avcbajbwrohw@##Q39uekvsmbvxc.fdjhd",
						Role: auth.Role{
							Type:  auth.RoleUIAdmin,
							Group: "sendcash-pay",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_empty_basic_auth",
			args: args{
				apiKeyAuth: []auth.APIKeyAuth{},
			},
			want:       nil,
			wantErr:    true,
			wantErrMsg: "no authentication data supplied for 'api_key_realm'",
		},

		{
			name: "should_error_for_nil_basic_auth",
			args: args{
				apiKeyAuth: nil,
			},
			want:       nil,
			wantErr:    true,
			wantErrMsg: "no authentication data supplied for 'api_key_realm'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAPIKeyRealm(tt.args.apiKeyAuth)

			if tt.wantErr {
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, got, tt.want)
		})
	}
}
