#!/usr/bin/env bash

#MISE description="Run E2E tests for job ID generation refactor"
#MISE dir="{{ config_root }}"

set -e

echo "🧪 Running Job ID E2E tests..."
echo ""

# Array of all job ID tests
tests=(
  "TestE2E_DirectEvent_AllSubscriptions"
  "TestE2E_DirectEvent_MustMatchSubscription"
  "TestE2E_FanOutEvent_AllSubscriptions"
  "TestE2E_FanOutEvent_MustMatchSubscription"
  "TestE2E_FormEndpoint_ContentType"
  "TestE2E_FormEndpoint_WithCustomHeaders"
  "TestE2E_OAuth2_SharedSecret"
  "TestE2E_OAuth2_ClientAssertion"
  "TestE2E_SingleEvent_JobID_Format"
  "TestE2E_SingleEvent_JobID_Deduplication"
  "TestE2E_FanoutEvent_JobID_Format"
  "TestE2E_FanoutEvent_MultipleOwners"
  "TestE2E_BroadcastEvent_JobID_Format"
  "TestE2E_BroadcastEvent_AllSubscribers"
  "TestE2E_DynamicEvent_JobID_Format"
  "TestE2E_DynamicEvent_MultipleEventTypes"
  "TestE2E_ReplayEvent_JobID_Format"
  "TestE2E_ReplayEvent_MultipleReplays"
  "TestE2E_BackupProjectData_MinIO"
  "TestE2E_BackupProjectData_OnPrem"
  "TestE2E_BackupProjectData_MultiTenant"
  "TestE2E_BackupProjectData_TimeFiltering"
  "TestE2E_BackupProjectData_AllTables"
  "TestE2E_AMQP_Single_BasicDelivery"
  "TestE2E_AMQP_Fanout_MultipleEndpoints"
  "TestE2E_AMQP_Broadcast_AllSubscribers"
  "TestE2E_AMQP_Single_EventTypeFilter"
  "TestE2E_AMQP_Single_WildcardEventType"
  "TestE2E_AMQP_Fanout_EventTypeFilter"
  "TestE2E_AMQP_Broadcast_EventTypeFilter"
  "TestE2E_AMQP_Single_BodyFilter_Equal"
  "TestE2E_AMQP_Single_BodyFilter_GreaterThan"
  "TestE2E_AMQP_Single_BodyFilter_In"
  "TestE2E_AMQP_Single_HeaderFilter"
  "TestE2E_AMQP_Single_CombinedFilters"
  "TestE2E_AMQP_Single_SourceBodyTransform"
  "TestE2E_AMQP_Single_SourceHeaderTransform"
  "TestE2E_AMQP_Single_NoMatchingSubscription"
  "TestE2E_AMQP_Single_InvalidEndpoint"
  "TestE2E_AMQP_Single_MissingEventType"
  "TestE2E_AMQP_Single_MalformedPayload"
  "TestE2E_AMQP_Fanout_MissingOwnerID"
  "TestE2E_AMQP_Single_FilterMismatch"
  "TestE2E_AMQP_Single_MultipleWorkers"
  "TestE2E_SQS_Single_BasicDelivery"
  "TestE2E_SQS_Fanout_MultipleEndpoints"
  "TestE2E_SQS_Broadcast_AllSubscribers"
  "TestE2E_SQS_Single_EventTypeFilter"
  "TestE2E_SQS_Single_WildcardEventType"
  "TestE2E_SQS_Fanout_EventTypeFilter"
  "TestE2E_SQS_Broadcast_EventTypeFilter"
  "TestE2E_SQS_Single_BodyFilter_Equal"
  "TestE2E_SQS_Single_BodyFilter_GreaterThan"
  "TestE2E_SQS_Single_BodyFilter_In"
  "TestE2E_SQS_Single_HeaderFilter"
  "TestE2E_SQS_Single_CombinedFilters"
  "TestE2E_SQS_Single_SourceBodyTransform"
  "TestE2E_SQS_Single_SourceHeaderTransform"
  "TestE2E_SQS_Single_NoMatchingSubscription"
  "TestE2E_SQS_Single_InvalidEndpoint"
  "TestE2E_SQS_Single_MissingEventType"
  "TestE2E_SQS_Single_MalformedPayload"
  "TestE2E_SQS_Fanout_MissingOwnerID"
  "TestE2E_SQS_Single_FilterMismatch"
  "TestE2E_SQS_Single_MultipleWorkers"
  "TestE2E_Kafka_Single_BasicDelivery"
  "TestE2E_Kafka_Fanout_MultipleEndpoints"
  "TestE2E_Kafka_Broadcast_AllSubscribers"
  "TestE2E_Kafka_Single_EventTypeFilter"
  "TestE2E_Kafka_Single_WildcardEventType"
  "TestE2E_Kafka_Fanout_EventTypeFilter"
  "TestE2E_Kafka_Broadcast_EventTypeFilter"
  "TestE2E_Kafka_Single_BodyFilter_Equal"
  "TestE2E_Kafka_Single_BodyFilter_GreaterThan"
  "TestE2E_Kafka_Single_BodyFilter_In"
  "TestE2E_Kafka_Single_HeaderFilter"
  "TestE2E_Kafka_Single_CombinedFilters"
  "TestE2E_Kafka_Single_SourceBodyTransform"
  "TestE2E_Kafka_Single_SourceHeaderTransform"
  "TestE2E_Kafka_Single_NoMatchingSubscription"
  "TestE2E_Kafka_Single_InvalidEndpoint"
  "TestE2E_Kafka_Single_MissingEventType"
  "TestE2E_Kafka_Single_MalformedPayload"
  "TestE2E_Kafka_Fanout_MissingOwnerID"
  "TestE2E_Kafka_Single_FilterMismatch"
  "TestE2E_Kafka_Single_MultipleWorkers"
)

# Counter for passed tests
passed=0
failed=0

# Run each test individually
for test in "${tests[@]}"; do
  echo "▶ Running: $test"
  if go test -v ./e2e/... -run "^${test}$" -timeout 2m; then
    echo "✅ $test passed"
    ((passed++))
  else
    echo "❌ $test failed"
    ((failed++))
  fi
  echo ""
done

# Summary
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 Test Summary:"
echo "   Passed: $passed"
echo "   Failed: $failed"
echo "   Total:  $((passed + failed))"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ $failed -eq 0 ]; then
  echo "✅ All Job ID E2E tests passed!"
  exit 0
else
  echo "❌ Some tests failed"
  exit 1
fi
