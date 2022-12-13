package policies

import "errors"

type AuthKey string

const AuthCtxKey AuthKey = "GoAuthzKey"

// ErrNotAllowed is returned when request is not permitted.
var ErrNotAllowed = errors.New("unauthorized to process request")
