package datastore

type EndpointListener interface {
	BeforeCreate(endpoint *Endpoint)
	AfterCreate(endpoint *Endpoint)
	BeforeUpdate(endpoint *Endpoint)
	AfterUpdate(endpoint *Endpoint)
}
