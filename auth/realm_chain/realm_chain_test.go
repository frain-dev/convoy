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
	APIKey: []config.APIKeyAuth{
		{
			APIKey: "avcbajbwrohw@##Q39uekvsmbvxc.fdjhd",
			Role: auth.Role{
				Type:  auth.RoleUIAdmin,
				Group: "sendcash-pay",
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
	fr.Name = "file_realm_1"

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
				AuthenticatedByRealm: fr.Name,
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
	fr.Name = "file_realm_1"

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
			wantErrMsg: fmt.Errorf("a realm with the name '%s' has already been registered", frClone.Name).Error(),
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
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Init(tt.args.authConfig); (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
