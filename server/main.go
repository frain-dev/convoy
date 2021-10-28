package server

// This is the main doc file, swag cli needs it to be named main.go

// TODO(daniel): we need a support url & email
// TODO(daniel): we need a public test api

// @title Convoy API Specification
// @version 0.1.12
// @description Convoy is a fast and secure distributed webhooks service. This document contains Convoy's API specification.
// @termsOfService https://convoy.io/terms

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email engineering@convoy.io

// @license.name Mozilla Public License 2.0
// @license.url https://www.mozilla.org/en-US/MPL/2.0/

// @schemes http https
// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

// @tag.name Application
// @tag.description Application related APIs

// @tag.name Application Endpoints
// @tag.description Endpoint related APIs

// @tag.name Messages
// @tag.description ApplicationEventrelated APIs

// @tag.name Group
// @tag.description Group related APIs

// Stub represents empty json or arbitrary json bodies for our doc annotations
type Stub struct{}
