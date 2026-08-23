# ![convoy image](./convoy_logo.png)

[![golangci-lint](https://github.com/frain-dev/convoy/actions/workflows/linter.yml/badge.svg)](https://github.com/frain-dev/convoy/actions/workflows/linter.yml)
[![Build and run all tests](https://github.com/frain-dev/convoy/actions/workflows/integration-tests.yml/badge.svg)](https://github.com/frain-dev/convoy/actions/workflows/integration-tests.yml)
[![GitHub stars](https://img.shields.io/github/stars/frain-dev/convoy)](https://github.com/frain-dev/convoy/stargazers)
[![License](https://img.shields.io/badge/License-Elastic%202.0-blue)](https://github.com/frain-dev/convoy/blob/main/LICENSE)
[![Release](https://img.shields.io/github/v/release/frain-dev/convoy)](https://github.com/frain-dev/convoy/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/getconvoy/convoy)](https://hub.docker.com/r/getconvoy/convoy)
[![Slack](https://img.shields.io/badge/Slack-Join_Community-4A154B?logo=slack&logoColor=white)](https://join.slack.com/t/convoy-community/shared_invite/zt-xiuuoj0m-yPp~ylfYMCV9s038QL0IUQ)
[![X](https://img.shields.io/badge/X-@getconvoy-black?logo=x&logoColor=white)](https://x.com/getconvoy)
[![LinkedIn](https://img.shields.io/badge/LinkedIn-Convoy-0A66C2?logo=linkedin&logoColor=white)](https://www.linkedin.com/company/convoy-webhooks/)

- Website: https://getconvoy.io
- Forum: [Convoy Community](https://community.getconvoy.io)
- Documentation: [docs.getconvoy.io](https://docs.getconvoy.io)
- Deploy:
  [Install Convoy](https://docs.getconvoy.io/deployment/install-convoy/docker)

[Convoy](https://getconvoy.io) is an open source high-performance webhooks
gateway used to securely ingest, persist, debug, deliver and manage millions of
events reliably with rich features such as retries, rate limiting, static ips,
circuit breaking, rolling secrets and more.

Convoy provides several key features:

- **Webhooks Gateway:** As a webhooks gateway, Convoy lives at the edge of your
  network to stream webhooks from your microservices, and send them out to your
  users as well as receive webhooks from your providers and route them to the
  required services. With this your internal systems are never exposed to the
  public internet.

- **Scalability:** Convoy acts as a dedicated message queue for webhooks, and
  was designed to be horizontally scalable. It includes two components, `server`
  and `agent`, which can be scaled independently to fit the need.

- **Security:** Convoy ships with several security features for webhooks, such
  as payload signing to ensure message integrity, bearer token authentication
  for authenticated webhook endpoints, OAuth2 and mTLS for endpoints that
  require them, and static ips for network environments with strict firewall
  rules.

- **Fan Out:** Convoy is able to route an event to multiple endpoints based on
  the event type or payload structure.

- **Rate Limiting:** While Convoy is able to ingest events at a massive rate, it
  throttles the delivery of these events to the endpoints at a configurable rate
  per endpoint.

- **Retries & Batch Retries:** Convoy supports two retry algorithms; constant
  time and exponential backoff with jitter. Where automatic retries are not
  sufficient, convoy provides batch retries for endpoints that have
  consecutively failed to process retried events.

- **Customer-Facing Dashboards:** Convoy allows you to generate customer facing
  webhooks dashboard to embed into your applications using an iframe. On this
  dashboard, users can debug webhooks, retry events, add endpoints, and
  configure each endpoint's subscription.

- **Endpoint Failure Notifications:** When endpoints consecutively fail to
  process events, convoy disables the endpoint and sends out a notification.
  Email, Slack, and Microsoft Teams are supported.

- **Search:** Find events by id, type, source, or idempotency key, or paste JSON
  to search the payload. Results stay in the date range you already have open.

- **Filters & Transforms:** Subscriptions can filter on body, headers, query,
  and path, and run a JS transform before delivery. Events that do not match
  are dropped.

- **Dynamic Endpoints:** Register one endpoint with a URL template and Convoy
  fills in the destination per event. `POST /events/dynamic` can wait until the
  event is matched before it returns 201.

- **Circuit Breaking:** Convoy opens the circuit when an endpoint fails
  consecutively, then probes it before sending traffic again.

- **SDKs:** Go, Python, JavaScript, Ruby, PHP, and Java clients are generated
  from the OpenAPI spec. Signature verification is hand-written and shared
  across languages.

- **Teams:** Invite people to an organisation and assign roles. Google SSO is
  available when it is enabled on the instance.

- **Postgres Queue:** The delivery queue can run on Postgres instead of Redis.
  This is experimental and needs a license.

- **Self-Hosted Premium:** Add a license key to enable paid features on your
  own instance. Community stays free for one user, one organisation, and two
  projects.

## Installation

- [Docker](https://docs.getconvoy.io/deployment/install-convoy/docker)
- [Kubernetes with Helm](https://docs.getconvoy.io/deployment/install-convoy/kubernetes)

## Contributing

Thank you for your interest in contributing! Please refer to
[CONTRIBUTING.md](https://github.com/frain-dev/convoy/blob/main/CONTRIBUTING.md)
for guidance. For contributions to the Convoy dashboard, please refer to the
[web/ui](https://github.com/frain-dev/convoy/tree/main/web/ui) directory.

## License

[Elastic License v2.0](https://github.com/frain-dev/convoy/blob/main/LICENSE)
