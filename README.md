Convoy
=========
[![golangci-lint](https://github.com/frain-dev/convoy/actions/workflows/linter.yml/badge.svg)](https://github.com/frain-dev/convoy/actions/workflows/linter.yml)
[![Build and run all tests](https://github.com/frain-dev/convoy/actions/workflows/go.yml/badge.svg)](https://github.com/frain-dev/convoy/actions/workflows/go.yml)
- Website: https://getconvoy.io
- Forum: [Github Discussions](https://github.com/frain-dev/convoy/discussions)
- Documentation: [getconvoy.io/docs](https://getconvoy.io/docs)
- Dowload: [getconvoy.io/download](https://getconvoy.io/download)
- Announcement: [Medium](https://medium.com/frain-technologies/tagged/convoy)
- Slack: [Slack](https://join.slack.com/t/convoy-community/shared_invite/zt-xiuuoj0m-yPp~ylfYMCV9s038QL0IUQ)

![convoy image](./convoy-logo.svg)

Convoy is a fast & secure webhooks service. It receives event data from a HTTP API and sends these event data to the configured endpoints. To get started download the [openapi spec](https://github.com/frain-dev/convoy/blob/main/docs/v3/openapi3.yaml) into Postman or Insomnia.

Convoy includes the following features: 

- **Security:** Convoy signs the payload of events, so applications ensure the events have not been tampered with. You can configure your desired hash function to use as well as the name of the header E.g. `X-Stripe-Signature` to enable backward comptabile migrations from custom built systems to Convoy.

- **URL per Events:** Convoy is able to receive one event and fan-out the event to multiple endpoints based on the configuration by the endpoint owner. On subscription, the endpoint owner configures what events should go to each endpoint. Overlaps are allowed.

- **Retries:** Convoy current supports two retry mechanism: Constant time retries and exponential backoff. You can configure which retry mechanism works best for your application.

- **Management UI**: Visibility and easy debugging are one of highly coverted features of a webhook delivery system. Convoy provides a UI to view your delivery attempt logs, filter by application, event status, date & time and perform flexible batch retries during downtimes.

- **Other features(Coming soon)**: Application Portal; enable you embed Convoy dashboard directly into your dashboard, Rate Limiting, Replay Attacks prevention, Multiple Ingest sources.

## Installation, Getting Started
Follow the instructions on our [quick start guide](https://getconvoy.io/docs/guide) to start publishing events with Convoy.

There are several ways of installing Convoy.

### Binaries
Convoy binaries can be downloaded with your package manager of choice. You can head over to [Downloads Page](https://getconvoy.io/download) to proceed.

### Docker images
Docker images are available on [Github Container Registry](https://github.com/frain-dev/convoy/pkgs/container/convoy).

You can launch a Convoy Container with the following

```bash
$ docker run \
	-p 5005:5005 \
	--name convoy-server \
	-v `pwd`/convoy.json:/convoy.json \
	packages.getconvoy.io/frain-dev/convoy:v0.4.9
```

You can view a sample configuration here - [convoy.json](https://github.com/frain-dev/convoy/blob/main/convoy.json.example).

### Building from source
To build Convoy from source code, you need:
* Go [version 1.16 or greater](https://golang.org/doc/install).
* NodeJS [version 14.17 or greater](https://nodejs.org).
* Npm [version 6 or greater](https://npmjs.com).

```bash
git clone https://github.com/frain-dev/convoy.git && cd convoy
go build -o convoy ./cmd
```

## Contributing
Thank you for your interest in contributing! Please refer to [CONTRIBUTING.md](https://github.com/frain-dev/convoy/blob/main/CONTRIBUTING.md) for guidance. For contributions to the Convoy dashboard, please refer to the [web/ui](https://github.com/frain-dev/convoy/tree/main/web/ui) directory.

## License
[Mozilla Public License v2.0](https://github.com/frain-dev/convoy/blob/main/LICENSE)
