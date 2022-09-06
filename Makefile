# init-hooks sets up git to recognise the .githooks directory as the hooks path for this repo
# it also makes all scripts in the .githooks folder executable
init-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/*

mockgen:
	go generate ./...

setup: init-hooks

ui_install: 
	cd web/ui/dashboard && \
	npm ci && \
 	 npm run build

build:
	scripts/build.sh

integration_tests:
	go test -tags integration -p 1 ./...

generate_migration_time:
	@date +"%Y%m%d%H%M%S"
