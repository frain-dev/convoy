package file

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
)

var fileRealmOpt = &config.FileRealmOption{
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
	APIKey: []auth.APIKeyAuth{
		{
			APIKey: "avcbajbwrohw@##Q39uekvsmbvxc.fdjhd",
			Role: auth.Role{
				Type:  auth.RoleUIAdmin,
				Group: "sendcash-pay",
			},
		},
	},
}

func TestFileRealm_Authenticate(t *testing.T) {

	fr, err := NewFileRealm(fileRealmOpt)
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
			name: "should_authenticate_apiKey_cred_successfully",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "avcbajbwrohw@##Q39uekvsmbvxc.fdjhd",
				},
			},
			want: &auth.AuthenticatedUser{
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
			got, err := fr.Authenticate(tt.args.cred)

			if tt.wantErr {
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, got, tt.want)
		})
	}
}

func TestNewFileRealm(t *testing.T) {
	type args struct {
		opt *config.FileRealmOption
	}
	tests := []struct {
		name       string
		args       args
		want       *FileRealm
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_initialize_file_realm_successfully",
			args: args{
				opt: fileRealmOpt,
			},
			want: &FileRealm{
				Basic: []BasicAuth{
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
				APIKey: []APIKeyAuth{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewFileRealm(tt.args.opt)

			if tt.wantErr {
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, got, tt.want)
		})
	}
}
