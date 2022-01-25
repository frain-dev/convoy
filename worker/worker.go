package worker

import "context"

type Worker interface {
	Start(context.Context)
}
