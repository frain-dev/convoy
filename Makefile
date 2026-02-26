# init-hooks sets up git to recognise the .githooks directory as the hooks path for this repo
# it also makes all scripts in the .githooks folder executable
init-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/*

mockgen:
	go generate ./...

setup: init-hooks

ui_install:
	scripts/ui.sh -b $(type)

build:
	scripts/build.sh

.PHONY: test
test:
	@go test -race -p 1 $(shell go list ./... | grep -v '/e2e') -timeout 30m

# Get Docker socket from active context if DOCKER_HOST is not set
DOCKER_HOST_VAL := $(or $(DOCKER_HOST),$(shell docker context inspect --format '{{.Endpoints.docker.Host}}' 2>/dev/null || echo ""))
DOCKER_CONTEXT := $(shell docker context show 2>/dev/null || echo "default")
DOCKER_SOCKET_PATH := $(shell echo "$(DOCKER_HOST_VAL)" | sed 's|^unix://||')
TEST_ENV := DOCKER_HOST="$(DOCKER_HOST_VAL)" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="$(DOCKER_SOCKET_PATH)"

# Fast E2E tests - Run on PRs (10-15 minutes)
.PHONY: test_e2e_fast
test_e2e_fast:
	@echo "Using docker context: $(DOCKER_CONTEXT) (DOCKER_HOST=$(DOCKER_HOST_VAL))"
	@echo "Running Fast E2E tests (non-pubsub)..."
	@echo "Running Direct Event tests..."
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_DirectEvent_AllSubscriptions
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_DirectEvent_MustMatchSubscription
	@echo "Running Fanout Event tests..."
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_FanOutEvent_AllSubscriptions
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_FanOutEvent_MustMatchSubscription
	@echo "Running Form Endpoint tests..."
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_FormEndpoint_ContentType
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_FormEndpoint_WithCustomHeaders
	@echo "Running OAuth2 tests..."
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_OAuth2_SharedSecret
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_OAuth2_ClientAssertion
	@echo "Running Job ID tests..."
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_SingleEvent_JobID_Format
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_SingleEvent_JobID_Deduplication
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_FanoutEvent_JobID_Format
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_FanoutEvent_MultipleOwners
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_BroadcastEvent_JobID_Format
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_BroadcastEvent_AllSubscribers
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_DynamicEvent_JobID_Format
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_DynamicEvent_MultipleEventTypes
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_ReplayEvent_JobID_Format
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_ReplayEvent_MultipleReplays
	@echo "Running Backup tests..."
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_BackupProjectData_MinIO
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_BackupProjectData_OnPrem
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_BackupProjectData_MultiTenant
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_BackupProjectData_TimeFiltering
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_BackupProjectData_AllTables
	@echo "✅ Fast E2E tests passed!"

# Slow PubSub/Message Broker tests - Run daily (60+ minutes)
.PHONY: test_e2e_pubsub
test_e2e_pubsub:
	@echo "Using docker context: $(DOCKER_CONTEXT) (DOCKER_HOST=$(DOCKER_HOST_VAL))"
	@echo "Running PubSub/Message Broker E2E tests..."
	@echo "Running AMQP PubSub tests..."
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_AMQP -timeout 30m
	@echo "Running SQS PubSub tests..."
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_SQS -timeout 30m
	@echo "Running Kafka PubSub tests..."
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_Kafka -timeout 30m
	@echo "Running Google Pub/Sub tests..."
	@$(TEST_ENV) go test -race -v ./e2e/... -run TestE2E_GooglePubSub -timeout 30m
	@echo "✅ All PubSub E2E tests passed!"

# Original test_e2e - runs ALL tests (for local comprehensive testing)
.PHONY: test_e2e
test_e2e: test_e2e_fast test_e2e_pubsub
	@echo "✅ All E2E tests (fast + pubsub) passed!"

# Run all E2E tests together (may be flaky, use test_e2e for CI)
test_e2e_all:
	@echo "Using docker context: $(DOCKER_CONTEXT) (DOCKER_HOST=$(DOCKER_HOST_VAL))"
	@$(TEST_ENV) go test -race -v ./e2e/...

# Run a specific E2E test
# Usage: make test_e2e_single TEST=TestE2E_DirectEvent_AllSubscriptions
test_e2e_single:
	@echo "Using docker context: $(DOCKER_CONTEXT) (DOCKER_HOST=$(DOCKER_HOST_VAL))"
	@$(TEST_ENV) go test -race -v ./e2e/... -run $(TEST)

generate_migration_time:
	@date +"%Y%m%d%H%M%S"

migrate_create:
	@go run cmd/main.go migrate create

generate_docs:
	swag init --generatedTime --parseDependency --parseInternal -d api/ api/*

run_dependencies:
	docker compose -f docker-compose.dep.yml up -d
