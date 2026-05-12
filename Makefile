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

# GO_TEST_RACE=0 skips the race detector (optional for local speed). Default 1; CI keeps race on for all triggers.
GO_TEST_RACE ?= 1
ifeq ($(GO_TEST_RACE),0)
GO_RACE_FLAG :=
else
GO_RACE_FLAG := -race
endif

# Package parallelism (lower under -race on small runners if you see OOM, e.g. TEST_PARALLEL=2).
TEST_PARALLEL ?= 4

# TEST_VERBOSE=0 drops -v to cut log volume in CI.
TEST_VERBOSE ?= 1
ifeq ($(TEST_VERBOSE),0)
TEST_VFLAG :=
else
TEST_VFLAG := -v
endif

.PHONY: test
test:
	@go test $(GO_RACE_FLAG) -p $(TEST_PARALLEL) $(shell go list ./... | grep -v '/e2e') $(TEST_VFLAG) -timeout 30m

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
	@echo "Running Direct / Fanout / Form / OAuth2 suites..."
	@$(TEST_ENV) go test $(GO_RACE_FLAG) $(TEST_VFLAG) ./e2e -timeout 30m -run 'TestE2E_(DirectEvent_AllSubscriptions|DirectEvent_MustMatchSubscription|FanOutEvent_AllSubscriptions|FanOutEvent_MustMatchSubscription|FormEndpoint_ContentType|FormEndpoint_WithCustomHeaders|OAuth2_SharedSecret|OAuth2_ClientAssertion)'
	@echo "Running Job ID suites..."
	@$(TEST_ENV) go test $(GO_RACE_FLAG) $(TEST_VFLAG) ./e2e -timeout 30m -run 'TestE2E_(SingleEvent_JobID_Format|SingleEvent_JobID_Deduplication|FanoutEvent_JobID_Format|FanoutEvent_MultipleOwners|BroadcastEvent_JobID_Format|BroadcastEvent_AllSubscribers|DynamicEvent_JobID_Format|DynamicEvent_MultipleEventTypes|ReplayEvent_JobID_Format|ReplayEvent_MultipleReplays)'
	@echo "Running Bulk Onboard suites..."
	@$(TEST_ENV) go test $(GO_RACE_FLAG) $(TEST_VFLAG) ./e2e -timeout 30m -run 'TestE2E_(BulkOnboard_JSON_CreatesEndpointsAndSubscriptions|BulkOnboard_CSV_CreatesEndpointsAndSubscriptions|BulkOnboard_DryRun_DoesNotCreateResources|BulkOnboard_DryRun_ReturnsValidationErrors|BulkOnboard_ValidationFailure_Returns400|BulkOnboard_EmptyItems_Returns400|BulkOnboard_MultipleBatches)'
	@echo "Running Backup tests..."
	@$(TEST_ENV) go test $(GO_RACE_FLAG) $(TEST_VFLAG) ./e2e/backup -timeout 15m
	@echo "✅ Fast E2E tests passed!"

# Slow PubSub/Message Broker tests - Run daily (60+ minutes)
.PHONY: test_e2e_pubsub
test_e2e_pubsub:
	@echo "Using docker context: $(DOCKER_CONTEXT) (DOCKER_HOST=$(DOCKER_HOST_VAL))"
	@echo "Running PubSub/Message Broker E2E tests..."
	@echo "Running AMQP PubSub tests..."
	@$(TEST_ENV) go test $(GO_RACE_FLAG) $(TEST_VFLAG) ./e2e/amqp -timeout 30m
	@echo "Running SQS PubSub tests..."
	@$(TEST_ENV) go test $(GO_RACE_FLAG) $(TEST_VFLAG) ./e2e/sqs -timeout 30m
	@echo "Running Kafka PubSub tests..."
	@$(TEST_ENV) go test $(GO_RACE_FLAG) $(TEST_VFLAG) ./e2e/kafka -timeout 30m
	@echo "Running Google Pub/Sub tests..."
	@$(TEST_ENV) go test $(GO_RACE_FLAG) $(TEST_VFLAG) ./e2e/pubsub -timeout 30m
	@echo "✅ All PubSub E2E tests passed!"

# Original test_e2e - runs ALL tests (for local comprehensive testing)
.PHONY: test_e2e
test_e2e: test_e2e_fast test_e2e_pubsub
	@echo "✅ All E2E tests (fast + pubsub) passed!"

# Run all E2E tests together (may be flaky, use test_e2e for CI)
test_e2e_all:
	@echo "Using docker context: $(DOCKER_CONTEXT) (DOCKER_HOST=$(DOCKER_HOST_VAL))"
	@$(TEST_ENV) go test $(GO_RACE_FLAG) $(TEST_VFLAG) ./e2e/...

# Run a specific E2E test
# Usage: make test_e2e_single TEST=TestE2E_DirectEvent_AllSubscriptions
test_e2e_single:
	@echo "Using docker context: $(DOCKER_CONTEXT) (DOCKER_HOST=$(DOCKER_HOST_VAL))"
	@$(TEST_ENV) go test $(GO_RACE_FLAG) $(TEST_VFLAG) ./e2e/... -run $(TEST)

generate_migration_time:
	@date +"%Y%m%d%H%M%S"

migrate_create:
	@go run cmd/main.go migrate create

generate_docs:
	@echo "Checking required tools (run 'mise install' to install all)..."
	@for tool in swag jq yq api-spec-converter openapi; do \
		command -v "$$tool" >/dev/null 2>&1 || { echo "❌ $$tool not found. Run 'mise install' to install all required tools."; exit 1; }; \
	done
	@echo "Generating docs..."
	go run docs/annotate_dtos/main.go
	swag init --generatedTime --parseDependency --parseDependencyLevel 3 --parseInternal -g handlers/main.go -d api/ api/*
	swag fmt -d ./api
	bash docs/fix_openapi_spec.sh
	api-spec-converter --from=swagger_2 --to=openapi_3 -s yaml ./docs/swagger.yaml > ./docs/v3/openapi3.yaml
	api-spec-converter --from=swagger_2 --to=openapi_3 ./docs/swagger.json > ./docs/v3/openapi3.json
	yq -i '.servers[0].description = "US Region" | .servers += [{"url": "https://eu.getconvoy.cloud/api", "description": "EU Region"}]' ./docs/v3/openapi3.yaml
	jq '.servers[0].description = "US Region" | .servers += [{"url": "https://eu.getconvoy.cloud/api", "description": "EU Region"}]' ./docs/v3/openapi3.json > ./docs/v3/openapi3.json.tmp && mv ./docs/v3/openapi3.json.tmp ./docs/v3/openapi3.json
	@echo "Validating specs..."
	openapi swagger validate ./docs/swagger.json
	openapi swagger validate ./docs/swagger.yaml
	openapi spec validate ./docs/v3/openapi3.yaml

run_dependencies:
	docker compose -f docker-compose.dep.yml up -d
