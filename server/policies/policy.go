package policies

import "errors"

type AuthKey string

const AuthCtxKey AuthKey = "GoAuthzKey"

var (
	// ErrNotAllowed is returned when request is not permitted.
	ErrNotAllowed = errors.New("Unauthorized to process request")
)
