package memorystore

type Row struct {
	key   string
	value interface{}
}

func NewRow(key string, value interface{}) *Row {
	return &Row{
		key:   key,
		value: value,
	}
}

func (r *Row) Key() string {
	return r.key
}

func (r *Row) Value() interface{} {
	return r.value
}
