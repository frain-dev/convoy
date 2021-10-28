package queue

import (
	"context"
	"io"
	"time"

	"github.com/frain-dev/convoy"
)

type Queuer interface {
	io.Closer
	Write(context.Context, convoy.TaskName, *convoy.Event, time.Duration) error
}

type Job struct {
	Err   error  `json:"err"`
	MsgID string `json:"msg_id"`
}
