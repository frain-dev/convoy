package convoy

import (
	"embed"
	"strings"
)

type HttpMethod string

type TaskName string

type CacheKey string

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

func (c CacheKey) Get(suffix string) CacheKey {
	var name strings.Builder
	delim := ":"

	name.WriteString(string(c))
	name.WriteString(delim)
	name.WriteString(suffix)

	return CacheKey(name.String())
}

func (c CacheKey) String() string {
	return string(c)
}

func ReadVersion() ([]byte, error) {
	data, err := f.ReadFile("VERSION")
	if err != nil {
		return nil, err
	}

	return data, nil
}

func GetVersion() string {
	v := "0.1.0"

	f, err := ReadVersion()
	if err != nil {
		return v
	}

	v = strings.TrimSuffix(string(f), "\n")
	return v
}

const (
	EventProcessor       TaskName = "EventProcessor"
	DeadLetterProcessor  TaskName = "DeadLetterProcessor"
	CreateEventProcessor TaskName = "CreateEventProcessor"
	ApplicationsCacheKey CacheKey = "applications"
	GroupsCacheKey       CacheKey = "groups"
)

const (
	StreamGroup           = "disq:"
	EventDeliveryIDLength = 12
)

const (
	// Number of messages reserved by a fetcher in the queue in one request.
	ReservationSize = 1000
	//Size of the internal buffer
	BufferSize = 100000
)
