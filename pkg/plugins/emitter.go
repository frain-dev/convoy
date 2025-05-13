package plugins

type Emitter interface {
	Emit(value any) error
}
