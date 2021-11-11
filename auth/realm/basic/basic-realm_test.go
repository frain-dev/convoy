package basic

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
)

func TestBasicRealm_GetName(t *testing.T) {
	ba := &BasicRealm{}
	require.Equal(t, ba.GetName(), "basic_realm")
}

func TestBasicRealm_Authenticate(t *testing.T) {
	basicAuth := []auth.BasicAuth{
		{
			Username: "username1",
			Password: "password1",
			Role: auth.Role{
				Type:  auth.RoleAdmin,
				Group: "sendcash-pay",
			},
		},
	}

	fr, err := NewBasicRealm(basicAuth)
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
			name: "should_authenticate_basic_cred_successfully",
			args: args{
				cred: &auth.Credential{
					Type:     auth.CredentialTypeBasic,
					Username: "username1",
					Password: "password1",
				},
			},
			want: &auth.AuthenticatedUser{
				AuthenticatedByRealm: "basic_realm",
				Credential: auth.Credential{
					Type:     auth.CredentialTypeBasic,
					Username: "username1",
					Password: "password1",
				},
				Role: auth.Role{
					Type:  auth.RoleAdmin,
					Group: "sendcash-pay",
				},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_wrong_username",
			args: args{
				cred: &auth.Credential{
					Type:     auth.CredentialTypeBasic,
					Username: "abc",
					Password: "password1",
				},
			},
			want:       nil,
			wantErr:    true,
			wantErrMsg: "credential not found",
		},
		{
			name: "should_error_for_wrong_password",
			args: args{
				cred: &auth.Credential{
					Type:     auth.CredentialTypeBasic,
					Username: "username1",
					Password: "abc",
				},
			},
			want:       nil,
			wantErr:    true,
			wantErrMsg: "credential not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fr.Authenticate(tt.args.cred)

			if tt.wantErr {
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNewBasicRealm(t *testing.T) {
	type args struct {
		basicAuth []auth.BasicAuth
	}
	tests := []struct {
		name       string
		args       args
		want       *BasicRealm
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_initialize_file_realm_successfully",
			args: args{
				basicAuth: []auth.BasicAuth{
					{
						Username: "username1",
						Password: "password1",
						Role: auth.Role{
							Type:  auth.RoleAdmin,
							Group: "sendcash-pay",
						},
					},
					{
						Username: "username2",
						Password: "password2",
						Role: auth.Role{
							Type:  auth.RoleUIAdmin,
							Group: "buycoins",
						},
					},
					{
						Username: "username3",
						Password: "password3",
						Role: auth.Role{
							Type:  auth.RoleSuperUser,
							Group: "paystack",
						},
					},
					{
						Username: "username4",
						Password: "password4",
						Role: auth.Role{
							Type:  auth.RoleAPI,
							Group: "termii",
						},
					},
				},
			},
			want: &BasicRealm{
				Basic: []auth.BasicAuth{
					{
						Username: "username1",
						Password: "password1",
						Role: auth.Role{
							Type:  auth.RoleAdmin,
							Group: "sendcash-pay",
						},
					},
					{
						Username: "username2",
						Password: "password2",
						Role: auth.Role{
							Type:  auth.RoleUIAdmin,
							Group: "buycoins",
						},
					},
					{
						Username: "username3",
						Password: "password3",
						Role: auth.Role{
							Type:  auth.RoleSuperUser,
							Group: "paystack",
						},
					},
					{
						Username: "username4",
						Password: "password4",
						Role: auth.Role{
							Type:  auth.RoleAPI,
							Group: "termii",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_empty_basic_auth",
			args: args{
				basicAuth: []auth.BasicAuth{},
			},
			want:       nil,
			wantErr:    true,
			wantErrMsg: "no authentication data supplied for 'basic_realm'",
		},

		{
			name: "should_error_for_nil_basic_auth",
			args: args{
				basicAuth: nil,
			},
			want:       nil,
			wantErr:    true,
			wantErrMsg: "no authentication data supplied for 'basic_realm'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewBasicRealm(tt.args.basicAuth)

			if tt.wantErr {
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, got, tt.want)
		})
	}
}
