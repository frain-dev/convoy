package convoy

import (
	"embed"
	"strings"
)

type HttpMethod string

type TaskName string

//go:embed VERSION
var f embed.FS

func (t TaskName) SetPrefix(prefix string) TaskName {
	var name strings.Builder
	delim := "-"

	name.WriteString(prefix)
	name.WriteString(delim)
	name.WriteString(string(t))

	return TaskName(name.String())
}

func ReadVersion() ([]byte, error) {
	data, err := f.ReadFile("VERSION")
	if err != nil {
		return nil, err
	}

	return data, nil
}

const (
	EventProcessor      TaskName = "EventProcessor"
	DeadLetterProcessor TaskName = "DeadLetterProcessor"
)

const (
	StreamGroup           = "taskq"
	EventDeliveryIDLength = 12
)
