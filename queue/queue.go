package queue

import (
	"context"
	"io"

	"github.com/hookcamp/hookcamp"
)

type Queuer interface {
	Read() chan Message
	io.Closer
	Write(context.Context, hookcamp.Message) error
}

type Message struct {
	Err  error            `json:"err"`
	Data hookcamp.Message `json:"data"`
}
