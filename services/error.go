package services

type ServiceError struct {
	errCode int
	errMsg  error
}

func NewServiceError(errCode int, errMsg error) *ServiceError {
	return &ServiceError{errCode: errCode, errMsg: errMsg}
}

func (s *ServiceError) Error() string {
	return s.errMsg.Error()
}

func (s *ServiceError) ErrCode() int {
	return s.errCode
}
