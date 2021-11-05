package realm_chain

import (
	"fmt"
	"testing"

	"github.com/frain-dev/convoy/auth/realm/file"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
)

func TestGet(t *testing.T) {
	require.Equal(t, rc, Get())
}

func TestRealmChain_Authenticate(t *testing.T) {
	rc = newRealmChain()

	fr, err := file.NewFileRealm("./testdata/file_realm_1.json")
	if err != nil {
		require.Nil(t, err)
		return
	}

	fr.Name = "file_realm_1"
	err = Get().RegisterRealm(fr)
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
			name: "should__authenticate_creds_with_file_realm",
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
			name: "should__authenticate_creds_with_file_realm",
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
			name: "should__authenticate_creds_with_file_realm",
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
			got, err := Get().Authenticate(tt.args.cred)

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
	rc = newRealmChain()

	fr, err := file.NewFileRealm("./testdata/file_realm_1.json")
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
			err := Get().RegisterRealm(tt.args.r)

			if tt.wantErr {
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
