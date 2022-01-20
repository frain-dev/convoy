package convoy

import "strings"

type HttpMethod string

type TaskName string

func (t TaskName) SetPrefix(prefix string) TaskName {
	var name strings.Builder
	delim := "-"

	name.WriteString(prefix)
	name.WriteString(delim)
	name.WriteString(string(t))

	return TaskName(name.String())
}

const (
	EventProcessor      TaskName = "EventProcessor"
	DeadLetterProcessor TaskName = "DeadLetterProcessor"
)
