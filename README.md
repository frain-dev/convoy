![convoy image](./convoy-logo.svg)
=========
[![golangci-lint](https://github.com/frain-dev/convoy/actions/workflows/linter.yml/badge.svg)](https://github.com/frain-dev/convoy/actions/workflows/linter.yml)
[![Build and run all tests](https://github.com/frain-dev/convoy/actions/workflows/go.yml/badge.svg)](https://github.com/frain-dev/convoy/actions/workflows/go.yml)
- Website: https://getconvoy.io
- Forum: [Github Discussions](https://github.com/frain-dev/convoy/discussions)
- Documentation: [getconvoy.io/docs](https://getconvoy.io/docs)
- Download: [getconvoy.io/download](https://getconvoy.io/download)
- Announcement: [Medium](https://medium.com/frain-technologies/tagged/convoy)
- Slack: [Slack](https://join.slack.com/t/convoy-community/shared_invite/zt-xiuuoj0m-yPp~ylfYMCV9s038QL0IUQ)


Convoy is a fast & secure webhooks proxy. It enables you to receive webhook events from providers and publish them to users.. To get started download the [openapi spec](https://github.com/frain-dev/convoy/blob/main/docs/v3/openapi3.yaml) into Postman or Insomnia.

Convoy includes the following features:

- **Webhooks Proxy:** Convoy enables you send webhooks to users, and helps you receive webhooks from your providers. It acts as a full proxy and lives at the edge of your network so you don't expose any of your internal systems or microservices.

- **Scalability:** Convoy acts as a dedicated message queue for webhooks. Its design enables you to horizontally scale out webhooks delivery.

-- **Security:** Convoy ships with several security features for webhooks, such as payload signing to ensure message integrity, endpoint authentication for authenticated routes, and static ips for network environments with strict firewall rules.

- **Fan Out:** Convoy is able to route an event to multiple endpoints based on the event type or payload structure. It relies on a subset of [MongoDB's Extended JSON v2](https://www.mongodb.com/docs/manual/reference/mongodb-extended-json/) to match event payload structure and route events respectively.

- **Rate Limiting:** While Convoy is able to ingest events at a large rate, it throttles the delivery of these events to the endpoints at a configurable rate per endpoint. 

- **Retries & Batch Retries:** Convoy currently supports two retry mechanisms: Constant time retries and exponential backoff. Once the retry limit is exhausted, you can batch retry any number of events when the endpoint is back up.

- **Management UI**: Visibility and easy debugging are one of highly coveted features of a webhook delivery system. Convoy provides a UI to view your delivery attempt logs, filter by application, event status, date & time and perform flexible batch retries during downtimes.

- **Customer-Facing Dashboards:** Convoy ships with out-of-the box webhooks dashboard that can be embedded into your dashboard with an iframe. With this, end-users can debug webhooks, retry and batch retry events easily.

- **Endpoints Failure Notification:** Convoy ships with failure notifications for endpoint failure. When an endpoint becomes dead, convoy sends out an email instantly to notify the endpoint owner of the failure.

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
