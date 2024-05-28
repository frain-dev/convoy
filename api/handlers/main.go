package handlers

// This is the main doc file, swag cli needs it to be named main.go

//	@title			Convoy API Reference
//	@version		24.1.4
//	@description	Convoy is a fast and secure webhooks proxy. This document contains datastore.s API specification.
//	@termsOfService	https://getconvoy.io/terms

//	@contact.name	Convoy Support
//	@contact.url	https://getconvoy.io/docs
//	@contact.email	support@getconvoy.io

//	@license.name	Mozilla Public License 2.0
//	@license.url	https://www.mozilla.org/en-US/MPL/2.0/

//	@schemes	https
//	@host		dashboard.getconvoy.io
//	@BasePath	/api

//	@securityDefinitions.apikey	ApiKeyAuth
//	@in							header
//	@name						Authorization

//	@tag.name			Organisations
//	@tag.description	Organisation related APIs

//	@tag.name			Subscriptions
//	@tag.description	Subscription related APIs

//	@tag.name			Endpoints
//	@tag.description	Endpoint related APIs

//	@tag.name			Events
//	@tag.description	Event related APIs

//	@tag.name			Sources
//	@tag.description	Source related APIs

//	@tag.name			Event Deliveries
//	@tag.description	EventDelivery related APIs

//	@tag.name			Delivery Attempts
//	@tag.description	Delivery Attempt related APIs

//	@tag.name			Projects
//	@tag.description	Project related APIs

//	@tag.name			Portal Links
//	@tag.description	Portal Links related APIs

//	@tag.name			Meta Events
//	@tag.description	Meta Events related APIs

// Stub represents empty json or arbitrary json bodies for our doc annotations
type Stub struct{}
