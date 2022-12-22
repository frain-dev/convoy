Convoy
=========
[![golangci-lint](https://github.com/frain-dev/convoy/actions/workflows/linter.yml/badge.svg)](https://github.com/frain-dev/convoy/actions/workflows/linter.yml)
[![Build and run all tests](https://github.com/frain-dev/convoy/actions/workflows/go.yml/badge.svg)](https://github.com/frain-dev/convoy/actions/workflows/go.yml)
- Website: https://getconvoy.io
- Forum: [Github Discussions](https://github.com/frain-dev/convoy/discussions)
- Documentation: [getconvoy.io/docs](https://getconvoy.io/docs)
- Download: [getconvoy.io/download](https://getconvoy.io/download)
- Announcement: [Medium](https://medium.com/frain-technologies/tagged/convoy)
- Slack: [Slack](https://join.slack.com/t/convoy-community/shared_invite/zt-xiuuoj0m-yPp~ylfYMCV9s038QL0IUQ)

![convoy image](./convoy-logo.svg)

Convoy is a fast & secure webhooks proxy. It enables you to receive webhook events from providers and publish them to users.. To get started download the [openapi spec](https://github.com/frain-dev/convoy/blob/main/docs/v3/openapi3.yaml) into Postman or Insomnia.

Convoy includes the following features:

- **Security:** Convoy signs the payload of events, so applications ensure the events have not been tampered with. You can configure your desired hash function to use as well as the name of the header E.g. `X-Stripe-Signature` to enable backward compatible migrations from custom-built systems to Convoy.

- **URL per Events:** Convoy is able to receive one event and fan-out the event to multiple endpoints based on the configuration by the endpoint owner. On subscription, the endpoint owner configures what events should go to each endpoint. Overlaps are allowed.

- **Retries:** Convoy currently supports two retry mechanisms: Constant time retries and exponential backoff. You can configure which retry mechanism works best for your application.

- **Management UI**: Visibility and easy debugging are one of highly coveted features of a webhook delivery system. Convoy provides a UI to view your delivery attempt logs, filter by application, event status, date & time and perform flexible batch retries during downtimes.

- **Application Portal**: Application Portal allows API providers embed Convoy dashboard directly into their API dashboard. With the API, users can build their own webhooks portal if you care so much about whitelisting. :)

## Installation, Getting Started
There are several ways to get started using Convoy.

### Option 1: Download our Binaries or Docker Image
Convoy binaries can be downloaded with your package manager of choice. You can head over to [Downloads Page](https://getconvoy.io/download) to proceed.

```bash
$ docker run \
	-p 5005:5005 \
	--name convoy-server \
    --network=host \
	-v `pwd`/convoy.json:/convoy.json \
	docker.cloudsmith.io/convoy/convoy/frain-dev/convoy:latest
```

### Option 2: Spin up an instance with third-party dependencies on a Linux VM
```bash
 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/frain-dev/convoy/main/deploy/vm-deploy.sh)"
```

### Option 3: If you don't wish to self-host
Sign up for a free [Convoy Cloud](https://dashboard.getconvoy.io/signup) account 


### Option 4: Building from source
To build Convoy from source code, you need:
* Go [version 1.16 or greater](https://golang.org/doc/install).
* NodeJS [version 14.17 or greater](https://nodejs.org).
* Npm [version 6 or greater](https://npmjs.com).

```bash
$ git clone https://github.com/frain-dev/convoy.git && cd convoy
$ make build
```

## Contributing
Thank you for your interest in contributing! Please refer to [CONTRIBUTING.md](https://github.com/frain-dev/convoy/blob/main/CONTRIBUTING.md) for guidance. For contributions to the Convoy dashboard, please refer to the [web/ui](https://github.com/frain-dev/convoy/tree/main/web/ui) directory.

## License
[Mozilla Public License v2.0](https://github.com/frain-dev/convoy/blob/main/LICENSE)
