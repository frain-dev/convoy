# Courier
Courier is a fast & secure webhooks service. It receives event data from a HTTP API and sends these event data to the configured endpoints.

Requirements
-----------------

Installation
-----------------


Getting Started
----------------

## hookcamp.json

For cluster authentication, the auth layer can use the following authentication mechanisms:
- None
- Basic Auth

For `None`, the json below will suffice:

```json
{
	"auth": {
           "type": "none"
	}
}
```

For `Basic Auth`, the json below will suffice:

```json
{
	"auth": {
            "type": "basic",
            "basic" : {
              "username": "username",
              "password": "password"
            }
        }
}
```
