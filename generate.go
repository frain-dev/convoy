package convoy

//go:generate mockgen --source datastore/repository.go --destination mocks/repository.go -package mocks
//go:generate mockgen --source queue/queue.go --destination mocks/queue.go -package mocks
//go:generate mockgen --source tracer/tracer.go --destination mocks/tracer.go -package mocks
//go:generate mockgen --source limiter/limiter.go --destination mocks/limiter.go -package mocks
//go:generate mockgen --source notification/notification.go --destination mocks/notification.go -package mocks
//go:generate mockgen --source cache/cache.go --destination mocks/cache.go -package mocks
