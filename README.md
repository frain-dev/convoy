![convoy image](./convoy-logo.svg)
=========
[![golangci-lint](https://github.com/frain-dev/convoy/actions/workflows/linter.yml/badge.svg)](https://github.com/frain-dev/convoy/actions/workflows/linter.yml)
[![Build and run all tests](https://github.com/frain-dev/convoy/actions/workflows/go.yml/badge.svg)](https://github.com/frain-dev/convoy/actions/workflows/go.yml)
- Website: https://getconvoy.io
- Forum: [Convoy Community](https://community.getconvoy.io)
- Documentation: [docs.getconvoy.io](https://docs.getconvoy.io)
- Deploy: [Install Convoy](https://docs.getconvoy.io/deployment/install-convoy/docker)
- Slack: [Join the Community](https://join.slack.com/t/convoy-community/shared_invite/zt-xiuuoj0m-yPp~ylfYMCV9s038QL0IUQ)


[Convoy](https://getconvoy.io) is an open source high-performance webhooks gateway used to securely ingest, persist, debug, deliver and manage millions of events reliably with rich features such as retries, rate limiting, static ips, circuit breaking, rolling secrets and more. 

Convoy provides several key features:

- **Webhooks Gateway:** As a webhooks gateway, Convoy lives at the edge of your network to stream webhooks from your microservices, and send them out to your users as well as receive webhooks from your providers and route them to the required services. With this your internal systems are never exposed to the public internet.

- **Scalability:** Convoy acts as a dedicated message queue for webhooks, and was designed to be horizontally scalable. It includes several components like the `api server`, `workers`, `scheduler`, and `socket server` which can be scaled independently to fit the need.

- **Security:** Convoy ships with several security features for webhooks, such as payload signing to ensure message integrity, bearer token authentication for authenticated webhook endpoints, and static ips for network environments with strict firewall rules.

- **Fan Out:** Convoy is able to route an event to multiple endpoints based on the event type or payload structure.

- **Rate Limiting:** While Convoy is able to ingest events at a massive rate, it throttles the delivery of these events to the endpoints at a configurable rate per endpoint. 

- **Retries & Batch Retries:** Convoy supports two retry algorithms; constant time and exponential backoff with jitter. Where automatic retries are not sufficient, convoy provides batch retries for endpoints are consecutively failed to process retried events.

- **Customer-Facing Dashboards:** Convoy allows you to generate customer facing webhooks dashboard to embed into your applications using an iframe. On this dashboard, users can debug webhooks, retry events, add endpoints, and configure each endpoint's subscription.

- **Endpoint Failure Notifications:** When endpoints consecutively fails to process events, convoy disables the endpoint and sends out a notification. Two types of notifications are supported: Email and Slack Notifications.

## Installation
- [Docker](https://docs.getconvoy.io/deployment/install-convoy/docker)
- [Kubernetes with Helm](https://docs.getconvoy.io/deployment/install-convoy/kubernetes)

## Contributing
Thank you for your interest in contributing! Please refer to [CONTRIBUTING.md](https://github.com/frain-dev/convoy/blob/main/CONTRIBUTING.md) for guidance. For contributions to the Convoy dashboard, please refer to the [web/ui](https://github.com/frain-dev/convoy/tree/main/web/ui) directory.

## License
[Mozilla Public License v2.0](https://github.com/frain-dev/convoy/blob/main/LICENSE)
