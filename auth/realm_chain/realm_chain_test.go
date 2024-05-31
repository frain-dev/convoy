package realm_chain

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/frain-dev/convoy/mocks"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm/file"
	"github.com/frain-dev/convoy/config"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	tests := []struct {
		name       string
		nFn        func(*atomic.Value)
		want       *RealmChain
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_get_successfully",
			nFn: func(v *atomic.Value) {
				rr := newRealmChain()
				rr.chain["abc"] = &file.FileRealm{}
				v.Store(rr)
			},
			want: &RealmChain{
				chain: chainMap{
					"abc": &file.FileRealm{},
				},
			},
			wantErr: false,
		},
		{
			name:       "should_error",
			nFn:        func(v *atomic.Value) { *v = atomic.Value{} },
			want:       nil,
			wantErr:    true,
			wantErrMsg: "call Init before this function",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.nFn != nil {
				tt.nFn(&realmChainSingleton)
			}

			got, err := Get()
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

var fileRealmOpt = &config.FileRealmOption{
	Basic: []config.BasicAuth{
		{
			Username: "username1",
			Password: "password1",
			Role: auth.Role{
				Type:    auth.RoleAdmin,
				Project: "sendcash-pay",
			},
		},
		{
			Username: "username2",
			Password: "password2",
			Role: auth.Role{
				Type:    auth.RoleAdmin,
				Project: "buycoins",
			},
		},
		{
			Username: "username3",
			Password: "password3",
			Role: auth.Role{
				Type:    auth.RoleSuperUser,
				Project: "paystack",
			},
		},
		{
			Username: "username4",
			Password: "password4",
			Role: auth.Role{
				Type:    auth.RoleAPI,
				Project: "termii",
			},
		},
	},
	APIKey: []config.APIKeyAuth{
		{
			APIKey: "avcbajbwrohw@##Q39uekvsmbvxc.fdjhd",
			Role: auth.Role{
				Type:    auth.RoleAdmin,
				Project: "sendcash-pay",
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
					Type:    auth.RoleAdmin,
					Project: "sendcash-pay",
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

			got, err := rc.Authenticate(context.Background(), tt.args.cred)

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
			wantErrMsg: fmt.Sprintf("a realm with the name '%s' has already been registered", frClone.GetName()),
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
					File: config.FileRealmOption{
						Basic: []config.BasicAuth{
							{
								Username: "test",
								Password: "test",
								Role: auth.Role{
									Type:    auth.RoleAPI,
									Project: "paystack",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAPIKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
			userRepo := mocks.NewMockUserRepository(ctrl)
			portalLinkRepo := mocks.NewMockPortalLinkRepository(ctrl)
			cache := mocks.NewMockCache(ctrl)
			err := Init(tt.args.authConfig, mockAPIKeyRepo, userRepo, portalLinkRepo, cache)
			if tt.wantErr {
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
