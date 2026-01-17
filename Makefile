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
	@for pkg in $$(go list ./... | grep -v '/e2e'); do \
    		go test -v -p 1 $$pkg; \
    	done

# E2E Tests - Run individually for reliability and reproducibility
test_e2e:
	@echo "Running E2E tests individually..."
	@go test -v ./e2e/... -run TestE2E_DirectEvent_AllSubscriptions -timeout 2m
	@go test -v ./e2e/... -run TestE2E_DirectEvent_MustMatchSubscription -timeout 2m
	@go test -v ./e2e/... -run TestE2E_FanOutEvent_AllSubscriptions -timeout 2m
	@go test -v ./e2e/... -run TestE2E_FanOutEvent_MustMatchSubscription -timeout 2m
	@go test -v ./e2e/... -run TestE2E_FormEndpoint_ContentType -timeout 2m
	@go test -v ./e2e/... -run TestE2E_FormEndpoint_WithCustomHeaders -timeout 2m
	@go test -v ./e2e/... -run TestE2E_OAuth2_SharedSecret -timeout 2m
	@go test -v ./e2e/... -run TestE2E_OAuth2_ClientAssertion -timeout 2m
	@echo "Running Job ID E2E tests..."
	@go test -v ./e2e/... -run TestE2E_SingleEvent_JobID_Format -timeout 2m
	@go test -v ./e2e/... -run TestE2E_SingleEvent_JobID_Deduplication -timeout 2m
	@go test -v ./e2e/... -run TestE2E_FanoutEvent_JobID_Format -timeout 2m
	@go test -v ./e2e/... -run TestE2E_FanoutEvent_MultipleOwners -timeout 2m
	@go test -v ./e2e/... -run TestE2E_BroadcastEvent_JobID_Format -timeout 2m
	@go test -v ./e2e/... -run TestE2E_BroadcastEvent_AllSubscribers -timeout 2m
	@go test -v ./e2e/... -run TestE2E_DynamicEvent_JobID_Format -timeout 2m
	@go test -v ./e2e/... -run TestE2E_DynamicEvent_MultipleEventTypes -timeout 2m
	@go test -v ./e2e/... -run TestE2E_ReplayEvent_JobID_Format -timeout 2m
	@go test -v ./e2e/... -run TestE2E_ReplayEvent_MultipleReplays -timeout 2m
	@echo "Running Backup E2E tests..."
	@go test -v ./e2e/... -run TestE2E_BackupProjectData_MinIO -timeout 2m
	@go test -v ./e2e/... -run TestE2E_BackupProjectData_OnPrem -timeout 2m
	@go test -v ./e2e/... -run TestE2E_BackupProjectData_MultiTenant -timeout 2m
	@go test -v ./e2e/... -run TestE2E_BackupProjectData_TimeFiltering -timeout 2m
	@go test -v ./e2e/... -run TestE2E_BackupProjectData_AllTables -timeout 2m
	@echo "Running AMQP PubSub E2E tests..."
	@go test -v ./e2e/... -run TestE2E_AMQP_Single_BasicDelivery -timeout 2m
	@go test -v ./e2e/... -run TestE2E_AMQP_Fanout_MultipleEndpoints -timeout 2m
	@go test -v ./e2e/... -run TestE2E_AMQP_Broadcast_AllSubscribers -timeout 2m
	@go test -v ./e2e/... -run TestE2E_AMQP_Single_EventTypeFilter -timeout 2m
	@go test -v ./e2e/... -run TestE2E_AMQP_Single_WildcardEventType -timeout 2m
	@go test -v ./e2e/... -run TestE2E_AMQP_Fanout_EventTypeFilter -timeout 2m
	@go test -v ./e2e/... -run TestE2E_AMQP_Broadcast_EventTypeFilter -timeout 2m
	@go test -v ./e2e/... -run TestE2E_AMQP_Single_BodyFilter_Equal -timeout 2m
	@go test -v ./e2e/... -run TestE2E_AMQP_Single_BodyFilter_GreaterThan -timeout 2m
	@go test -v ./e2e/... -run TestE2E_AMQP_Single_BodyFilter_In -timeout 2m
	@go test -v ./e2e/... -run TestE2E_AMQP_Single_HeaderFilter -timeout 2m
	@go test -v ./e2e/... -run TestE2E_AMQP_Single_CombinedFilters -timeout 2m
	@echo "✅ All E2E tests passed!"

# Run all E2E tests together (may be flaky, use test_e2e for CI)
test_e2e_all:
	go test -v ./e2e/... -timeout 10m

# Run a specific E2E test
# Usage: make test_e2e_single TEST=TestE2E_DirectEvent_AllSubscriptions
test_e2e_single:
	go test -v ./e2e/... -run $(TEST) -timeout 2m

generate_migration_time:
	@date +"%Y%m%d%H%M%S"

generate_docs:
	swag init --generatedTime --parseDependency --parseInternal -d api/ api/*

run_dependencies:
	docker compose -f docker-compose.dep.yml up -d
