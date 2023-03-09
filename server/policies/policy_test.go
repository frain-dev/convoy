package policies

import (
	"github.com/frain-dev/convoy/auth"
	"github.com/stretchr/testify/require"
)

type basetest struct {
	name          string
	authCtx       *auth.AuthenticatedUser
	assertion     require.ErrorAssertionFunc
	expectedError error
}
