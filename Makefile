# init-hooks sets up git to recognise the .githooks directory as the hooks path for this repo
# it also makes all scripts in the .githooks folder executable
init-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/*

mockgen:
	go generate ./...

ui_install:
	cd web/ui/dashboard && \
	npm install && \
       	npm run build && \
	mv dist/* ../../../server/ui/build
