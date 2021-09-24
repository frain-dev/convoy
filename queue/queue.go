package queue

import (
	"context"
	"io"

	"github.com/frain-dev/convoy"
)

type Queuer interface {
	Read() chan Message
	io.Closer
	Write(context.Context, convoy.Message) error
}

type Message struct {
	Err  error          `json:"err"`
	Data convoy.Message `json:"data"`
}
