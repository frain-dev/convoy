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

integration_tests:
	go run ./cmd migrate up
	go test -tags integration -p 1 ./...

docker_e2e_tests:
	go test -tags docker_testcon -p 1 ./...

generate_migration_time:
	@date +"%Y%m%d%H%M%S"

generate_docs:
	swag init --generatedTime --parseDependency --parseInternal -d api/ api/*

run_dependencies:
	docker compose -f docker-compose.dep.yml up -d
