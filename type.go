package convoy

import (
	"embed"
	"fmt"
	"strings"
)

type HttpMethod string

type TaskName string

type QueueName string

type CacheKey string

type GenericMap map[string]interface{}

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
	EventProcessor         TaskName = "EventProcessor"
	DeadLetterProcessor    TaskName = "DeadLetterProcessor"
	CreateEventProcessor   TaskName = "CreateEventProcessor"
	NotificationProcessor  TaskName = "NotificationProcessor"
	IndexDocument          TaskName = "index document"
	DailyAnalytics         TaskName = "daily analytics"
	MonitorTwitterSources  TaskName = "monitor twitter sources"
	RetentionPolicies      TaskName = "retention_policies"
	EmailProcessor         TaskName = "EmailProcessor"
	ExpireSecretsProcessor TaskName = "ExpireSecretsProcessor"
	EndpointsCacheKey      CacheKey = "endpoints"
	ProjectsCacheKey       CacheKey = "projects"
	TokenCacheKey          CacheKey = "tokens"
	SourceCacheKey         CacheKey = "sources"
)

// queues
const (
	EventQueue       QueueName = "EventQueue"
	CreateEventQueue QueueName = "CreateEventQueue"
	PriorityQueue    QueueName = "PriorityQueue"
	ScheduleQueue    QueueName = "ScheduleQueue"
	DefaultQueue     QueueName = "DefaultQueue"
)

// Exports dir
const (
	DefaultOnPremDir = "/var/convoy/export"
	TmpExportDir     = "/tmp/convoy/export"
)

const (
	EventDeliveryIDLength = 12
)

const (
	Concurrency = 100
)

var ErrUnsupportedDatebase = fmt.Errorf("unsupported database for search detected, remove search configuration or use a supported database (mongodb)")
