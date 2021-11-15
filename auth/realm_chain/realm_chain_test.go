package realm_chain

import (
	"fmt"
	"testing"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm/file"
	"github.com/frain-dev/convoy/config"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	rr := newRealmChain()
	rr.chain["abc"] = &file.FileRealm{}

	realmChainSingleton.Store(rr)

	rc, err := Get()
	if err != nil {
		require.Nil(t, err)
		return
	}

	require.Equal(t, rr, rc)
}

var fileRealmOpt = &config.FileRealmOption{
	Basic: []config.BasicAuth{
		{
			Username: "username1",
			Password: "password1",
			Role: auth.Role{
				Type:   auth.RoleAdmin,
				Groups: []string{"sendcash-pay"},
			},
		},
		{
			Username: "username2",
			Password: "password2",
			Role: auth.Role{
				Type:   auth.RoleUIAdmin,
				Groups: []string{"buycoins"},
			},
		},
		{
			Username: "username3",
			Password: "password3",
			Role: auth.Role{
				Type:   auth.RoleSuperUser,
				Groups: []string{"paystack"},
			},
		},
		{
			Username: "username4",
			Password: "password4",
			Role: auth.Role{
				Type:   auth.RoleAPI,
				Groups: []string{"termii"},
			},
		},
	},
	APIKey: []config.APIKeyAuth{
		{
			APIKey: "avcbajbwrohw@##Q39uekvsmbvxc.fdjhd",
			Role: auth.Role{
				Type:   auth.RoleUIAdmin,
				Groups: []string{"sendcash-pay"},
			},
		},
	},
}

func TestRealmChain_Authenticate(t *testing.T) {
	realmChainSingleton.Store(newRealmChain())

	fr, err := file.NewFileRealm(fileRealmOpt)
	if err != nil {
		require.Nil(t, err)
		return
	}

	rc, err := Get()
	if err != nil {
		require.Nil(t, err)
		return
	}

	err = rc.RegisterRealm(fr)
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
			name: "should_authenticate_creds_with_file_realm",
			args: args{
				cred: &auth.Credential{
					Type:     auth.CredentialTypeBasic,
					Username: "username1",
					Password: "password1",
				},
			},
			want: &auth.AuthenticatedUser{
				AuthenticatedByRealm: fr.GetName(),
				Credential: auth.Credential{
					Type:     auth.CredentialTypeBasic,
					Username: "username1",
					Password: "password1",
				},
				Role: auth.Role{
					Type:   auth.RoleAdmin,
					Groups: []string{"sendcash-pay"},
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
			wantErrMsg: ErrAuthFailed.Error(),
			wantErr:    true,
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
			wantErrMsg: ErrAuthFailed.Error(),
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc, err := Get()
			if err != nil {
				require.Nil(t, err)
				return
			}

			got, err := rc.Authenticate(tt.args.cred)

			if tt.wantErr {
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, got, tt.want)
		})
	}
}

func TestRealmChain_RegisterRealm(t *testing.T) {
	realmChainSingleton.Store(newRealmChain())

	fr, err := file.NewFileRealm(fileRealmOpt)
	if err != nil {
		require.Nil(t, err)
		return
	}
	frClone := *fr
	type args struct {
		r auth.Realm
	}

	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_register_realm_successfully",
			args: args{
				r: fr,
			},
			wantErr: false,
		},
		{
			name: "should_error_for_duplicate_realm_name",
			args: args{
				r: &frClone,
			},
			wantErrMsg: fmt.Errorf("a realm with the name '%s' has already been registered", frClone.GetName()).Error(),
			wantErr:    true,
		},
		{
			name: "should_error_for_nil_realm",
			args: args{
				r: nil,
			},
			wantErrMsg: ErrNilRealm.Error(),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc, err := Get()
			if err != nil {
				require.Nil(t, err)
				return
			}

			err = rc.RegisterRealm(tt.args.r)
			if tt.wantErr {
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestInit(t *testing.T) {
	type args struct {
		authConfig *config.AuthConfiguration
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_init_successfully",
			args: args{
				authConfig: &config.AuthConfiguration{
					RequireAuth: true,
					File: config.FileRealmOption{
						Basic: []config.BasicAuth{
							{
								Username: "test",
								Password: "test",
								Role: auth.Role{
									Type:   auth.RoleAPI,
									Groups: []string{"paystack"},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_allow_empty_group_for_superuser",
			args: args{
				authConfig: &config.AuthConfiguration{
					RequireAuth: true,
					File: config.FileRealmOption{
						Basic: []config.BasicAuth{
							{
								Username: "test",
								Password: "test",
								Role: auth.Role{
									Type:   auth.RoleSuperUser,
									Groups: nil,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_empty_username",
			args: args{
				authConfig: &config.AuthConfiguration{
					RequireAuth: true,
					File: config.FileRealmOption{
						Basic: []config.BasicAuth{
							{
								Username: "",
								Password: "test",
								Role: auth.Role{
									Type:   auth.RoleAPI,
									Groups: []string{"paystack"},
								},
							},
						},
					},
				},
			},
			wantErr:    true,
			wantErrMsg: "username and password are required for basic auth config",
		},
		{
			name: "should_error_for_empty_password",
			args: args{
				authConfig: &config.AuthConfiguration{
					RequireAuth: true,
					File: config.FileRealmOption{
						Basic: []config.BasicAuth{
							{
								Username: "test",
								Password: "",
								Role: auth.Role{
									Type:   auth.RoleAPI,
									Groups: []string{"paystack"},
								},
							},
						},
					},
				},
			},
			wantErr:    true,
			wantErrMsg: "username and password are required for basic auth config",
		},
		{
			name: "should_error_for_invalid_role_type",
			args: args{
				authConfig: &config.AuthConfiguration{
					RequireAuth: true,
					File: config.FileRealmOption{
						Basic: []config.BasicAuth{
							{
								Username: "test",
								Password: "test",
								Role: auth.Role{
									Type:   "abc",
									Groups: []string{"paystack"},
								},
							},
						},
					},
				},
			},
			wantErr:    true,
			wantErrMsg: "invalid role type: abc",
		},
		{
			name: "should_error_for_nil_groups",
			args: args{
				authConfig: &config.AuthConfiguration{
					RequireAuth: true,
					File: config.FileRealmOption{
						Basic: []config.BasicAuth{
							{
								Username: "test",
								Password: "test",
								Role: auth.Role{
									Type:   auth.RoleAPI,
									Groups: nil,
								},
							},
						},
					},
				},
			},
			wantErr:    true,
			wantErrMsg: "please specify groups for basic auth",
		},
		{
			name: "should_error_for_empty_groups",
			args: args{
				authConfig: &config.AuthConfiguration{
					RequireAuth: true,
					File: config.FileRealmOption{
						Basic: []config.BasicAuth{
							{
								Username: "test",
								Password: "test",
								Role: auth.Role{
									Type:   auth.RoleAPI,
									Groups: []string{},
								},
							},
						},
					},
				},
			},
			wantErr:    true,
			wantErrMsg: "please specify groups for basic auth",
		},
		{
			name: "should_error_for_empty_group_name",
			args: args{
				authConfig: &config.AuthConfiguration{
					RequireAuth: true,
					File: config.FileRealmOption{
						Basic: []config.BasicAuth{
							{
								Username: "test",
								Password: "test",
								Role: auth.Role{
									Type:   auth.RoleAPI,
									Groups: []string{""},
								},
							},
						},
					},
				},
			},
			wantErr:    true,
			wantErrMsg: "empty group name not allowed for basic auth",
		},

		{
			name: "should_init_with_api_key_config",
			args: args{
				authConfig: &config.AuthConfiguration{
					RequireAuth: true,
					File: config.FileRealmOption{
						APIKey: []config.APIKeyAuth{
							{
								APIKey: "1234567",
								Role: auth.Role{
									Type:   auth.RoleAPI,
									Groups: []string{"paystack"},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_empty_api_key",
			args: args{
				authConfig: &config.AuthConfiguration{
					RequireAuth: true,
					File: config.FileRealmOption{
						APIKey: []config.APIKeyAuth{
							{
								APIKey: "",
								Role: auth.Role{
									Type:   auth.RoleAPI,
									Groups: []string{"paystack"},
								},
							},
						},
					},
				},
			},
			wantErr:    true,
			wantErrMsg: "api-key is required for api-key auth config",
		},
		{
			name: "should_error_for_invalid_role_type",
			args: args{
				authConfig: &config.AuthConfiguration{
					RequireAuth: true,
					File: config.FileRealmOption{
						APIKey: []config.APIKeyAuth{
							{
								APIKey: "123456",
								Role: auth.Role{
									Type:   "abc",
									Groups: []string{"paystack"},
								},
							},
						},
					},
				},
			},
			wantErr:    true,
			wantErrMsg: "invalid role type: abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Init(tt.args.authConfig)
			if tt.wantErr {
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
