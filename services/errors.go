package services

const (
	ErrCodeAuthInvalid    = "auth.invalid"
	ErrCodeLicenseExpired = "license.expired"
)

const (
	ErrMutualTLSFeatureUnavailable = "mutual TLS feature unavailable, please upgrade your license"
	ErrOAuth2FeatureUnavailable    = "OAuth2 feature unavailable, please upgrade your license"
)

type ServiceError struct {
	ErrMsg string
	Err    error
	Code   string // e.g. "auth.invalid", "license.expired"
}

func (a *ServiceError) Error() string {
	return a.ErrMsg
}

func (a *ServiceError) Unwrap() error {
	return a.Err
}
