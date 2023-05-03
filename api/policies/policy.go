package policies

import (
	"errors"

	"github.com/frain-dev/convoy/api/types"
)

const AuthUserCtx types.ContextKey = "authUser"

// ErrNotAllowed is returned when request is not permitted.
var ErrNotAllowed = errors.New("unauthorized to process request")
