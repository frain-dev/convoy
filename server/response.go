package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
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
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"` // TODO(subomi,daniel): this makes the endpoint doc vauge
}

func newServerResponse(msg string, object interface{}, statusCode int) serverResponse {
	data, err := json.Marshal(object)
	if err != nil {
		log.Errorf("Unable to marshal response data - %s", err)
	}
	return newServerResponseWithStatus(true, msg, data, statusCode)
}

func newServerResponseWithStatus(status bool, msg string, data json.RawMessage, statusCode int) serverResponse {
	return serverResponse{
		Status:  status,
		Message: msg,
		Data:    data,
		Response: Response{
			StatusCode: statusCode,
		},
	}
}
