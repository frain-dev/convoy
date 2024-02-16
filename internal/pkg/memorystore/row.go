package memorystore

type Row struct {
	key         string
	value       interface{}
	resetSignal <-chan bool
}

func NewRow(key string, value interface{}) *Row {
	return &Row{
		key:         key,
		value:       value,
		resetSignal: make(<-chan bool),
	}
}

func (r *Row) Key() string {
	return r.key
}

func (r *Row) Value() interface{} {
	return r.value
}

func (r *Row) ResetSignal() <-chan bool {
	return r.resetSignal
}
