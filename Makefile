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
	scripts/build.sh -b $(type)

integration_tests:
	go run ./cmd migrate up
	go test -tags integration -p 1 ./...

generate_migration_time:
	@date +"%Y%m%d%H%M%S"

generate_docs:
	swag init --generatedTime --parseDependency --parseInternal -d api/ api/*

run_dependencies:
	docker compose -f docker-compose.dep.yml up -d

IMAGE ?= ghcr.io/elenpay/ep-backend/convoy
VERSION ?= $(shell git describe --tags --abbrev=0)

ui_build:
	cd web/ui/dashboard && npm i && npm run build
	mkdir -p api/ui/build
	cp -r web/ui/dashboard/dist/* api/ui/build/

binary_linux_amd64:
	docker run --rm -v "$(shell pwd)":/src -w /src \
		-e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=amd64 \
		golang:1.25 sh -c "go mod download && go build -o convoy-amd64 ./cmd"

binary_linux_arm64:
	docker run --rm -v "$(shell pwd)":/src -w /src \
		-e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=arm64 \
		golang:1.25 sh -c "go mod download && go build -o convoy-arm64 ./cmd"

binary_linux: binary_linux_amd64 binary_linux_arm64

docker_build_push_amd64:
	cp convoy-amd64 convoy
	docker build --platform linux/amd64 -f release.Dockerfile \
		-t $(IMAGE):$(VERSION)-amd64 --push .
	rm -f convoy

docker_build_push_arm64:
	cp convoy-arm64 convoy
	docker build --platform linux/arm64 -f release.Dockerfile \
		-t $(IMAGE):$(VERSION)-arm64 --push .
	rm -f convoy

docker_manifest:
	docker buildx imagetools create \
		-t $(IMAGE):$(VERSION) \
		$(IMAGE):$(VERSION)-amd64 \
		$(IMAGE):$(VERSION)-arm64

docker_release: ui_build binary_linux docker_build_push_amd64 docker_build_push_arm64 docker_manifest
	rm -f convoy-amd64 convoy-arm64
