# Convoy
=========
- Website: https://getconvoy.io
- Forum: [Github Discussions](https://github.com/frain-dev/convoy/discussions)
- Documentation: [getconvoy.io/docs](https://getconvoy.io/docs)
- Announcement: [Medium]()
- Slack: [Slack]()

![convoy image](./convoy-logo.svg)

Convoy is a fast & secure webhooks service. It receives event data from a HTTP API and sends these event data to the configured endpoints. To get started download the [openapi spec](https://github.com/frain-dev/convoy/blob/main/openapi.yaml) into Postman or Insomnia.

It includes the following features
- **Sign payload:** Configure hash function to use in signing payload.
- **Retry events:** Retry events to endpoints.
- **Delivery Attempt Logs:** View request headers and body as well as response headers and body.
- **Rich UI**: To easily debug and retry failed events.

## Install

There are various ways of installing Convoy.

### Precompiled binaries
Precompiled binaries for released versions are available in the [releases section](https://github.com/frain-dev/convoy/releases)
on [Github](https://github.com/frain-dev/convoy).

### Docker images
Docker images are available on [Github Container Registry](https://github.com/frain-dev/convoy/pkgs/container/convoy).

You can launch a Convoy Container to try it out with 

```bash
$ docker run \
	-p 5005:5005 \
	--name convoy-server \ 
	-v `pwd`/convoy.json:convoy.json \
	ghcr.io/frain-dev/convoy
```

You can download a sample configuration of [convoy.json](https://github.com/frain-dev/convoy/blob/main/convoy.json).


### Building from source
To build Convoy from source code, you need:
* Go [version 1.16 or greater](https://golang.org/doc/install).
* NodeJS [version 14.17 or greater](https://nodejs.org).
* Npm [version 6 or greater](https://npmjs.com).

```bash
git clone https://github.com/frain-dev/convoy.git
cd convoy
go build -o convoy ./convoy
```

## Concepts

1. **Apps:** An app is an abstraction representing a user who wants to receive webhooks. Currently, an app contains one endpoint to receive webhooks.
2. **Events:** An event represents a webhook event to be sent to an app.
3. **Delivery Attempts:** A delivery attempt represents an attempt to send an event to it's respective app's endpoint. It contains the `event body`, `status code` and `response body` received on attempt. The amount of attempts on a failed delivery depends on your configured retry strategy.

## Configuration

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
		"type": "none"
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

-   You can set basic auth mechanism with the following:

```json
{
	"auth": {
		"type": "basic",
		"basic": {
			"username": "username",
			"password": "password"
		}
	}
}
```

## License
[Mozilla Public License v2.0](https://github.com/frain-dev/convoy/blob/main/LICENSE)