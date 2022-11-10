package policies

import "errors"

var (
	AuthCtxKey = "GoAuthzKey"
)

var (
	// ErrNotAllowed is returned when request is not permitted.
	ErrNotAllowed = errors.New("Unauthorized to process request")
)
