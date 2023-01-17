package pubsub

type PubSub interface {
	Dispatch()
	Listen()
	Stop()
}