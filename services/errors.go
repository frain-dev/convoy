package services

type ServiceError struct {
	errMsg string
	err    error
}

func (a *ServiceError) Error() string {
	return a.errMsg
}

func (a *ServiceError) Unwrap() error {
	return a.err
}
