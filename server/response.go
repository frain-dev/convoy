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

func newErrorResponse(msg string, statusCode int) serverResponse {
	return serverResponse{
		Status:  false,
		Message: msg,
		Response: Response{
			StatusCode: statusCode,
		},
	}
}

type serverResponse struct {
	Response
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func newServerResponse(msg string, data interface{}, statusCode int) serverResponse {
	return newServerResponseWithStatus(true, msg, data, statusCode)
}

func newServerResponseWithStatus(status bool, msg string, data interface{}, statusCode int) serverResponse {
	return serverResponse{
		Status:  status,
		Message: msg,
		Data:    data,
		Response: Response{
			StatusCode: statusCode,
		},
	}
}
