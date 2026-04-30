package tracer

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/frain-dev/convoy"
)

// Tracer scope names — passed to TracerProvider.Tracer to identify the
// emitting library. Each top-level Convoy package has one. Always reference
// the constant; never use a string literal at the call site, so a global
// rename stays grep-safe.
const (
	TracerNameServices = "github.com/frain-dev/convoy/services"
	TracerNameWorker   = "github.com/frain-dev/convoy/worker"
)

// Span names — full strings, layered prefix (api.*, services.*, repository.*,
// queue.*, worker.*, dispatcher.*) baked in. Do NOT build a span name from
// concatenation; add a new constant here instead. The trace UI's grouping,
// dashboards, and alerts all key off the literal name, so renames must be
// deliberate.
const (
	// services.*
	SpanServicesEventCreateFanout  = "services.event.create_fanout"
	SpanServicesEventCreateDynamic = "services.event.create_dynamic"
	SpanServicesEventCreateMeta    = "services.event.create_meta"
	SpanServicesEventDeliveryRetry = "services.event_delivery.retry"

	// worker.task.* — one per convoy.TaskName the consumer dispatches on.
	// SpanForTaskName below maps the runtime TaskName to the right constant.
	SpanWorkerTaskProcessEventDelivery            = "worker.task.process_event_delivery"
	SpanWorkerTaskProcessRetryEventDelivery       = "worker.task.process_retry_event_delivery"
	SpanWorkerTaskProcessEventCreation            = "worker.task.process_event_creation"
	SpanWorkerTaskProcessDynamicEventCreation     = "worker.task.process_dynamic_event_creation"
	SpanWorkerTaskProcessBroadcastEventCreation   = "worker.task.process_broadcast_event_creation"
	SpanWorkerTaskMatchEventSubscriptions         = "worker.task.match_event_subscriptions"
	SpanWorkerTaskProcessMetaEvent                = "worker.task.process_meta_event"
	SpanWorkerTaskProcessNotification             = "worker.task.process_notification"
	SpanWorkerTaskProcessEmail                    = "worker.task.process_email"
	SpanWorkerTaskExpireSecrets                   = "worker.task.expire_secrets"
	SpanWorkerTaskMonitorTwitterSources           = "worker.task.monitor_twitter_sources"
	SpanWorkerTaskTokenizeSearch                  = "worker.task.tokenize_search"
	SpanWorkerTaskTokenizeSearchForProject        = "worker.task.tokenize_search_for_project"
	SpanWorkerTaskRetentionPolicies               = "worker.task.retention_policies"
	SpanWorkerTaskEnqueueBackupJobs               = "worker.task.enqueue_backup_jobs"
	SpanWorkerTaskProcessBackupJob                = "worker.task.process_backup_job"
	SpanWorkerTaskManualBackupJob                 = "worker.task.manual_backup_job"
	SpanWorkerTaskDailyAnalytics                  = "worker.task.daily_analytics"
	SpanWorkerTaskStreamCliEvents                 = "worker.task.stream_cli_events"
	SpanWorkerTaskDeleteArchivedTasks             = "worker.task.delete_archived_tasks"
	SpanWorkerTaskBatchRetry                      = "worker.task.batch_retry"
	SpanWorkerTaskBulkOnboard                     = "worker.task.bulk_onboard"
	SpanWorkerTaskUpdateOrganisationStatus        = "worker.task.update_organisation_status"
	SpanWorkerTaskRefreshMetricsMaterializedViews = "worker.task.refresh_metrics_materialized_views"
	SpanWorkerTaskUnknown                         = "worker.task.unknown"
)

// taskNameSpans maps every convoy.TaskName to its span constant. Adding a new
// task means adding both the const above and an entry here; the consumer
// middleware will fall back to SpanWorkerTaskUnknown otherwise.
var taskNameSpans = map[convoy.TaskName]string{
	convoy.EventProcessor:                   SpanWorkerTaskProcessEventDelivery,
	convoy.RetryEventProcessor:              SpanWorkerTaskProcessRetryEventDelivery,
	convoy.CreateEventProcessor:             SpanWorkerTaskProcessEventCreation,
	convoy.CreateDynamicEventProcessor:      SpanWorkerTaskProcessDynamicEventCreation,
	convoy.CreateBroadcastEventProcessor:    SpanWorkerTaskProcessBroadcastEventCreation,
	convoy.MatchEventSubscriptionsProcessor: SpanWorkerTaskMatchEventSubscriptions,
	convoy.MetaEventProcessor:               SpanWorkerTaskProcessMetaEvent,
	convoy.NotificationProcessor:            SpanWorkerTaskProcessNotification,
	convoy.EmailProcessor:                   SpanWorkerTaskProcessEmail,
	convoy.ExpireSecretsProcessor:           SpanWorkerTaskExpireSecrets,
	convoy.MonitorTwitterSources:            SpanWorkerTaskMonitorTwitterSources,
	convoy.TokenizeSearch:                   SpanWorkerTaskTokenizeSearch,
	convoy.TokenizeSearchForProject:         SpanWorkerTaskTokenizeSearchForProject,
	convoy.RetentionPolicies:                SpanWorkerTaskRetentionPolicies,
	convoy.EnqueueBackupJobs:                SpanWorkerTaskEnqueueBackupJobs,
	convoy.ProcessBackupJob:                 SpanWorkerTaskProcessBackupJob,
	convoy.ManualBackupJob:                  SpanWorkerTaskManualBackupJob,
	convoy.DailyAnalytics:                   SpanWorkerTaskDailyAnalytics,
	convoy.StreamCliEventsProcessor:         SpanWorkerTaskStreamCliEvents,
	convoy.DeleteArchivedTasksProcessor:     SpanWorkerTaskDeleteArchivedTasks,
	convoy.BatchRetryProcessor:              SpanWorkerTaskBatchRetry,
	convoy.BulkOnboardProcessor:             SpanWorkerTaskBulkOnboard,
	convoy.UpdateOrganisationStatus:         SpanWorkerTaskUpdateOrganisationStatus,
	convoy.RefreshMetricsMaterializedViews:  SpanWorkerTaskRefreshMetricsMaterializedViews,
}

// SpanForTaskName returns the span name constant that should wrap a worker
// handler for the given TaskName. Unknown task names get SpanWorkerTaskUnknown
// — the consumer still records a span, but with a flag-able name so adding a
// new task without a corresponding constant is visible at trace-review time.
func SpanForTaskName(t convoy.TaskName) string {
	if name, ok := taskNameSpans[t]; ok {
		return name
	}
	return SpanWorkerTaskUnknown
}

// Event names — attached to spans via span.AddEvent. These mark a status
// change or sub-step on an active span; they do NOT create child spans. Same
// rule as span names: never use a literal at the call site.
const (
	// event.delivery.*
	EventEventDeliveryError     = "event.delivery.error"
	EventEventDeliverySuccess   = "event.delivery.success"
	EventEventDeliveryDiscarded = "event.delivery.discarded"
	EventEventDeliveryInfo      = "event.delivery.info"

	// event.retry.delivery.*
	EventEventRetryDeliveryError          = "event.retry.delivery.error"
	EventEventRetryDeliverySuccess        = "event.retry.delivery.success"
	EventEventRetryDeliveryDiscarded      = "event.retry.delivery.discarded"
	EventEventRetryDeliveryRateLimited    = "event.retry.delivery.rate_limited"
	EventEventRetryDeliveryCircuitBreaker = "event.retry.delivery.circuit_breaker"

	// event.subscription.matching.*
	EventEventSubscriptionMatchingError     = "event.subscription.matching.error"
	EventEventSubscriptionMatchingSuccess   = "event.subscription.matching.success"
	EventEventSubscriptionMatchingDuplicate = "event.subscription.matching.duplicate"

	// event.creation.*
	EventEventCreationError   = "event.creation.error"
	EventEventCreationSuccess = "event.creation.success"

	// dynamic.event.*
	EventDynamicEventCreationError             = "dynamic.event.creation.error"
	EventDynamicEventCreationSuccess           = "dynamic.event.creation.success"
	EventDynamicEventSubscriptionMatchingError = "dynamic.event.subscription.matching.error"
	EventDynamicEventSubscriptionMatchingOK    = "dynamic.event.subscription.matching.success"

	// broadcast.event.*
	EventBroadcastEventCreationError      = "broadcast.event.creation.error"
	EventBroadcastEventCreationSuccess    = "broadcast.event.creation.success"
	EventBroadcastSubscriptionMatchingErr = "broadcast.subscription.matching.error"
	EventBroadcastSubscriptionMatchingOK  = "broadcast.subscription.matching.success"

	// meta_event.*
	EventMetaEventDelivery = "meta_event_delivery"

	// dispatcher detailed-trace events (attached to the active outbound HTTP
	// span emitted by otelhttp). Names match Go's httptrace hooks for easy
	// cross-reference in the trace UI.
	EventDispatcherDNSLookupStart    = "dns_lookup_start"
	EventDispatcherDNSLookupDone     = "dns_lookup_done"
	EventDispatcherConnectStart      = "connect_start"
	EventDispatcherConnectDone       = "connect_done"
	EventDispatcherTLSHandshakeStart = "tls_handshake_start"
	EventDispatcherTLSHandshakeDone  = "tls_handshake_done"
	EventDispatcherFirstByte         = "first_byte"
)

// Attribute keys for Convoy-domain identifiers attached to spans. Use these
// rather than ad-hoc strings so dashboards and queries stay consistent across
// services.
const (
	AttrProjectID         = attribute.Key("convoy.project_id")
	AttrOwnerID           = attribute.Key("convoy.owner_id")
	AttrEventID           = attribute.Key("convoy.event_id")
	AttrEndpointID        = attribute.Key("convoy.endpoint_id")
	AttrSubscriptionID    = attribute.Key("convoy.subscription_id")
	AttrEventDeliveryID   = attribute.Key("convoy.event_delivery_id")
	AttrDeliveryAttemptID = attribute.Key("convoy.delivery_attempt_id")
	AttrTaskName          = attribute.Key("convoy.task_name")
)

// RecordError sets the span status from an error and records the error on the
// span. nil errors flip the span to Ok. Safe to call on a nil span.
func RecordError(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err == nil {
		span.SetStatus(codes.Ok, "")
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// AddEvent attaches a named event with attributes to the active span in ctx.
// Always pass a constant from this file as `name`; never a string literal.
// No-op when the active span isn't recording.
func AddEvent(ctx context.Context, name string, attrs map[string]interface{}) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	if len(attrs) == 0 {
		span.AddEvent(name)
		return
	}
	kvs := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		switch x := v.(type) {
		case string:
			kvs = append(kvs, attribute.String(k, x))
		case int:
			kvs = append(kvs, attribute.Int(k, x))
		case int64:
			kvs = append(kvs, attribute.Int64(k, x))
		case float64:
			kvs = append(kvs, attribute.Float64(k, x))
		case bool:
			kvs = append(kvs, attribute.Bool(k, x))
		}
	}
	span.AddEvent(name, trace.WithAttributes(kvs...))
}
