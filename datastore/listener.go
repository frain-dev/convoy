package datastore

type EndpointListener interface {
	AfterCreate(endpoint *Endpoint)
	AfterUpdate(endpoint *Endpoint)
}
