package server

type EndpointError struct {
	Err        error
	StatusCode int
}

func (e *EndpointError) Error() string {
	return e.Err.Error()
}
