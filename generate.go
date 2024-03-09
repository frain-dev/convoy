package convoy

//go:generate mockgen --source datastore/repository.go --destination mocks/repository.go -package mocks
//go:generate mockgen --source queue/queue.go --destination mocks/queue.go -package mocks
//go:generate mockgen --source internal/pkg/limiter/limiter.go --destination mocks/limiter.go -package mocks
//go:generate mockgen --source cache/cache.go --destination mocks/cache.go -package mocks
//go:generate mockgen --source internal/pkg/smtp/smtp.go --destination mocks/smtp.go -package mocks
//go:generate mockgen --source internal/pkg/socket/socket.go --destination mocks/socket.go -package mocks
//go:generate mockgen --source internal/pkg/pubsub/pubsub.go --destination mocks/pubsub.go -package mocks
//go:generate mockgen --source internal/pkg/dedup/dedup.go --destination mocks/dedup.go -package mocks
