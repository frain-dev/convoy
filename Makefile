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

test:
	go test -p 1 $(shell go list ./... | grep -v '/e2e')

# E2E Tests - Run individually for reliability and reproducibility
test_e2e:
	@echo "Running E2E tests individually for maximum reliability..."
	@go test -v ./e2e/... -run TestE2E_DirectEvent_AllSubscriptions -timeout 2m
	@go test -v ./e2e/... -run TestE2E_DirectEvent_MustMatchSubscription -timeout 2m
	@go test -v ./e2e/... -run TestE2E_FanOutEvent_AllSubscriptions -timeout 2m
	@go test -v ./e2e/... -run TestE2E_FanOutEvent_MustMatchSubscription -timeout 2m
	@go test -v ./e2e/... -run TestE2E_FormEndpoint_ContentType -timeout 2m
	@go test -v ./e2e/... -run TestE2E_FormEndpoint_WithCustomHeaders -timeout 2m
	@go test -v ./e2e/... -run TestE2E_OAuth2_SharedSecret -timeout 2m
	@go test -v ./e2e/... -run TestE2E_OAuth2_ClientAssertion -timeout 2m
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
