package convoy

import (
	"embed"
	"strings"
)

const (
	AuthUserCtx     ContextKey = "authUser"
	PageableCtx     ContextKey = "pageable"
	ProjectCtx      ContextKey = "project"
	OrganisationCtx ContextKey = "organisation"
	RequestIDKey    ContextKey = "requestId"
)

type ContextKey string

type HttpMethod string

type TaskName string

type QueueName string

type CacheKey string

var Version string

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
	if Version != "" {
		return strings.TrimSpace(Version)
	}
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
	EventProcessor                   TaskName = "EventProcessor"
	RetryEventProcessor              TaskName = "RetryEventProcessor"
	CreateEventProcessor             TaskName = "CreateEventProcessor"
	CreateDynamicEventProcessor      TaskName = "CreateDynamicEventProcessor"
	CreateBroadcastEventProcessor    TaskName = "CreateBroadcastEventProcessor"
	MetaEventProcessor               TaskName = "MetaEventProcessor"
	NotificationProcessor            TaskName = "NotificationProcessor"
	TokenizeSearch                   TaskName = "TokenizeSearch"
	TokenizeSearchForProject         TaskName = "TokenizeProjectSearch"
	DailyAnalytics                   TaskName = "DailyAnalytics"
	StreamCliEventsProcessor         TaskName = "StreamCliEventsProcessor"
	MonitorTwitterSources            TaskName = "MonitorTwitterSources"
	RetentionPolicies                TaskName = "RetentionPolicies"
	EnqueueBackupJobs                TaskName = "EnqueueBackupJobs"
	ProcessBackupJob                 TaskName = "ProcessBackupJob"
	EmailProcessor                   TaskName = "EmailProcessor"
	ExpireSecretsProcessor           TaskName = "ExpireSecretsProcessor"
	DeleteArchivedTasksProcessor     TaskName = "DeleteArchivedTasksProcessor"
	MatchEventSubscriptionsProcessor TaskName = "MatchEventSubscriptionsProcessor"
	BatchRetryProcessor              TaskName = "BatchRetryProcessor"
	BulkOnboardProcessor             TaskName = "BulkOnboardProcessor"
	UpdateOrganisationStatus         TaskName = "UpdateOrganisationStatus"
	RefreshMetricsMaterializedViews  TaskName = "RefreshMetricsMaterializedViews"

	TokenCacheKey   CacheKey = "tokens"
	ProjectCacheKey CacheKey = "projects"
)

// queues
const (
	EventQueue         QueueName = "EventQueue"
	CreateEventQueue   QueueName = "CreateEventQueue"
	MetaEventQueue     QueueName = "MetaEventQueue"
	RetryEventQueue    QueueName = "RetryEventQueue"
	StreamQueue        QueueName = "StreamQueue"
	ScheduleQueue      QueueName = "ScheduleQueue"
	DefaultQueue       QueueName = "DefaultQueue"
	EventWorkflowQueue QueueName = "EventWorkflowQueue"
	BatchRetryQueue    QueueName = "BatchRetryQueue"
)

// Exports dir
const (
	DefaultOnPremDir = "/var/convoy/export"
	TmpExportDir     = "/tmp/convoy/export"
)
