package util

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/frain-dev/convoy/pkg/log"
	"github.com/go-chi/render"
)

type Response struct {
	StatusCode int `json:"-"`
}

func (res Response) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, res.StatusCode)
	return nil
}

func NewErrorResponse(msg string, statusCode int) ServerResponse {
	return ServerResponse{
		Status:  false,
		Message: msg,
		Response: Response{
			StatusCode: statusCode,
		},
	}
}

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

func NewServiceErrResponse(err error) ServerResponse {
	msg := ""
	statusCode := http.StatusBadRequest
	switch v := err.(type) {
	case *ServiceError:
		msg = v.Error()
		statusCode = v.ErrCode()
	case error:
		msg = v.Error()
	}

	return ServerResponse{
		Status:  false,
		Message: msg,
		Response: Response{
			StatusCode: statusCode,
		},
	}
}

type ServerResponse struct {
	Response
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func NewServerResponse(msg string, object interface{}, statusCode int) ServerResponse {
	data, err := json.Marshal(object)
	if err != nil {
		log.Errorf("Unable to marshal response data - %s", err)
	}
	return newServerResponseWithStatus(true, msg, data, statusCode)
}

func newServerResponseWithStatus(status bool, msg string, data json.RawMessage, statusCode int) ServerResponse {
	return ServerResponse{
		Status:  status,
		Message: msg,
		Data:    data,
		Response: Response{
			StatusCode: statusCode,
		},
	}
}

func WriteResponse(w http.ResponseWriter, r *http.Request, v []byte, status int) {
	render.Status(r, status)

	buf := bytes.NewBuffer(v)

	w.Header().Set("Content-Type", "application/json")
	if status, ok := r.Context().Value(render.StatusCtxKey).(int); ok {
		w.WriteHeader(status)
	}
	w.Write(buf.Bytes()) //nolint:errcheck
}
