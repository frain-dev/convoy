package convoy

//go:generate mockgen --source group.go --destination mocks/group.go -package mocks
//go:generate mockgen --source application.go --destination mocks/application.go -package mocks
//go:generate mockgen --source event.go --destination mocks/event.go -package mocks
//go:generate mockgen --source event_delivery.go --destination mocks/event_delivery.go -package mocks
//go:generate mockgen --source queue/queue.go --destination mocks/queue.go -package mocks
//go:generate mockgen --source tracer/tracer.go --destination mocks/tracer.go -package mocks
