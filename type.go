package convoy

import (
	"embed"
	"strings"
)

type HttpMethod string

type TaskName string

type QueueName string

type CacheKey string

//go:embed VERSION
var F embed.FS

//go:embed sql/*.sql
var MigrationFiles embed.FS

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

func readVersion(fs embed.FS) ([]byte, error) {
	data, err := fs.ReadFile("VERSION")
	if err != nil {
		return nil, err
	}

	return data, nil
}

// TODO(subomi): This needs to be refactored for everywhere we depend
// on this code.
func GetVersion() string {
	v := "0.1.0"

	f, err := readVersion(F)
	if err != nil {
		return v
	}

	v = strings.TrimSpace(string(f))
	return v
}

func GetVersionFromFS(fs embed.FS) string {
	v := "0.1.0"

	f, err := readVersion(fs)
	if err != nil {
		return v
	}

	v = strings.TrimSpace(string(f))
	return v
}

const (
	EventProcessor              TaskName = "EventProcessor"
	CreateEventProcessor        TaskName = "CreateEventProcessor"
	CreateDynamicEventProcessor TaskName = "CreateDynamicEventProcessor"
	MetaEventProcessor          TaskName = "MetaEventProcessor"
	NotificationProcessor       TaskName = "NotificationProcessor"
	// todo(raymond): remove this and it's references
	IndexDocument                TaskName = "index document"
	TokenizeSearch               TaskName = "tokenize search"
	TokenizeSearchForProject     TaskName = "tokenize search for project"
	DailyAnalytics               TaskName = "daily analytics"
	StreamCliEventsProcessor     TaskName = "StreamCliEventsProcessor"
	MonitorTwitterSources        TaskName = "monitor twitter sources"
	RetentionPolicies            TaskName = "retention_policies"
	EmailProcessor               TaskName = "EmailProcessor"
	ExpireSecretsProcessor       TaskName = "ExpireSecretsProcessor"
	DeleteArchivedTasksProcessor TaskName = "DeleteArchivedTasksProcessor"

	EndpointsCacheKey          CacheKey = "endpoints"
	OrganisationsCacheKey      CacheKey = "organisations"
	OrganisationMemberCacheKey CacheKey = "organisation_members"
	ProjectsCacheKey           CacheKey = "projects"
	SubscriptionsCacheKey      CacheKey = "subscriptions"
	TokenCacheKey              CacheKey = "tokens"
	SourceCacheKey             CacheKey = "sources"
)

// queues
const (
	EventQueue       QueueName = "EventQueue"
	CreateEventQueue QueueName = "CreateEventQueue"
	MetaEventQueue   QueueName = "MetaEventQueue"
	SearchIndexQueue QueueName = "SearchIndexQueue"
	StreamQueue      QueueName = "StreamQueue"
	ScheduleQueue    QueueName = "ScheduleQueue"
	DefaultQueue     QueueName = "DefaultQueue"
)

// Exports dir
const (
	DefaultOnPremDir = "/var/convoy/export"
	TmpExportDir     = "/tmp/convoy/export"
)
