package server

import (
	"net/http"

	"github.com/go-chi/render"
)

type Response struct {
	StatusCode int `json:"-"`
}

func (res Response) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, res.StatusCode)
	return nil
}

type errorResponse struct {
	Response
	Message string `json:"message"`
}

func newErrorResponse(msg string, statusCode int) errorResponse {
	return errorResponse{
		Message: msg,
		Response: Response{
			StatusCode: statusCode,
		},
	}
}
