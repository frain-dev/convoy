# Convoy
Convoy is a fast & secure webhooks service. It receives event data from a HTTP API and sends these event data to the configured endpoints.

Installation
-----------------
You can either download the docker image or compile from source


##### Docker 
```bash
docker pull ghcr.io/frain-dev/convoy:v0.1.0
```

##### Compile from source
```bash
git clone https://github.com/frain-dev/convoy.git
cd convoy
go build -o convoy ./cmd
```

Concepts
-----------------
1. **Apps:** An app is an abstraction representing a user who wants to receive webhooks. Currently, an app contains one endpoint to receive webhooks.
2. **Events:** An event represents a webhook event to be sent to an app.
3. **Delivery Attempts:** A delivery attempt represents an attempt to send an event to it's respective app's endpoint. It contains the `event body`, `status code` and `response body` received on attempt. The amount of attempts on a failed delivery depends on your configured retry strategy.


How it Works
-----------------

Configuration
-----------------
Convoy is configured using a json file with a sample configuration below: 
```json
{
  "database": {
    "dsn": "mongo-url-with-username-and-password"
  },
  "queue": {
    "type": "redis",
    "redis": {
      "dsn": "redis-url-with-username-and-password"
    }
  },
  "server": {
    "http": {
      "port": 5005
    }
  },
  "auth": {
    "type": "none",
  },
  "strategy": {
    "type": "default",
    "default": {
      "intervalSeconds": 125,
      "retryLimit": 15
    }
  },
  "signature": {
    "header": "X-Company-Event-Webhook-Signature"
  }
}
```
#### Notes to Configuration
- You can set basic auth mechanism with the following:
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
