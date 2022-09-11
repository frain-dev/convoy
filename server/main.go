package server

// This is the main doc file, swag cli needs it to be named main.go

// @title Convoy API Specification
// @version 0.1.12
// @description Convoy is a fast and secure distributed webhooks service. This document contains datastore.s API specification.
// @termsOfService https://getconvoy.io/terms

// @contact.name API Support
// @contact.url https://getconvoy.io/docs
// @contact.email Info@frain.dev

// @license.name Mozilla Public License 2.0
// @license.url https://www.mozilla.org/en-US/MPL/2.0/

// @schemes https
// @host dashboard.getconvoy.io
// @BasePath /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

// @tag.name User
// @tag.description User related APIs

// @tag.name Application
// @tag.description Application related APIs

// @tag.name Source
// @tag.description Source related APIs

// @tag.name Subscriptions
// @tag.description Subscription related APIs

// @tag.name Application Endpoints
// @tag.description Endpoint related APIs

// @tag.name Events
// @tag.description Event related APIs

// @tag.name APIKey
// @tag.description API Key related APIs

// @tag.name EventDelivery
// @tag.description EventDelivery related APIs

// @tag.name DeliveryAttempts
// @tag.description Delivery Attempt related APIs

// @tag.name Group
// @tag.description Group related APIs

// @tag.name Organisation
// @tag.description Organisation related APIs

// Stub represents empty json or arbitrary json bodies for our doc annotations
type Stub struct{}
