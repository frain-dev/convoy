package drain

import "context"

// Fence is the store authority barrier. Bind provisions an explicitly enabled
// namespace; inspection must never call it.
type Fence interface {
	Bind(context.Context) error
	Check(context.Context) error
	Close(context.Context, Operation) error
	Open(context.Context, Operation) error
	Closed(context.Context, Operation) (bool, error)
}
