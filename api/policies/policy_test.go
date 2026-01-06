package policies

import (
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
)

type basetest struct {
	name          string
	authCtx       *auth.AuthenticatedUser
	assertion     require.ErrorAssertionFunc
	expectedError error
}
