package policies

import "github.com/frain-dev/convoy/auth"

type basetest struct {
	name          string
	authCtx       *auth.AuthenticatedUser
	wantErr       bool
	expectedError error
}
