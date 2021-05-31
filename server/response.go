package server

import (
	"net/http"
	"time"

	"github.com/go-chi/render"
)

type response struct {
	StatusCode int
	Timestamp  int64
}

func (res response) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, res.StatusCode)
	return nil
}

type errorResponse struct {
	response
	Message string `json:"message"`
}

func newErrorResponse(msg string, statusCode int) errorResponse {
	return errorResponse{
		Message: msg,
		response: response{
			StatusCode: statusCode,
			Timestamp:  time.Now().Unix(),
		},
	}
}
