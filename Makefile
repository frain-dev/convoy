# init-hooks sets up git to recognise the .githooks directory as the hooks path for this repo
# it also makes all scripts in the .githooks folder executable
init-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/*

mockgen:
	mockgen -source=event_delivery.go -destination=./mocks/event_delivery.go -package=mocks
	mockgen -source=event.go -destination=./mocks/event.go -package=mocks
	mockgen -source=group.go -destination=./mocks/group.go -package=mocks
	mockgen -source=application.go -destination=./mocks/application.go -package=mocks
	mockgen -source=queue/queue.go -destination=./mocks/queue.go -package=mocks