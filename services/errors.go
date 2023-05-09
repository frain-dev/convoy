package services

type ServiceError struct {
	ErrMsg string
	Err    error
}

func (a *ServiceError) Error() string {
	return a.ErrMsg
}

func (a *ServiceError) Unwrap() error {
	return a.Err
}
