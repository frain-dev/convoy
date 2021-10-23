---
title: Overview
description: 'Your list of apps on Convoys'
id: overview
---

## What is Convoy?
Convoy is a fast & secure webhooks service. It receives event data from a HTTP API and sends these event data to the configured endpoints.

## Glossary
This section collects brief definitions of some of the technical terms used in the documentation for Convoy.

### Applications
An application represents a user's application trying to receive webhooks. Once you create an application on Convoy. You receive an `app_id` that you should save and supply in subsequent API calls to perform other actions E.g. Send an event. Currently, an application maps to one endpoint. In the future, an application should map to multiple endpoints. When you creating an application, you should supply a [secret](#secrets).

### Endpoints
An endpoint represents a target URL to receive events. Endpoint can be in either of this states - `active`, `inactive` or `pending`. When an endpoint is in the `inactive` state all events sent will be saved and discarded until the endpoint is brought back up.

### Events
An event represents a specific event triggered by your system. Convoy persists events sent to dead endpoints with a status - `Discarded`. This enables users re-activate their endpoints and easily retry discarded events without the need to re-trigger the events from your systems.

### Delivery Attempts
A delivery attempt represents a single attempt to deliver an event to an endpoint. Specifically, it contains 2 things - Request Headers & Payload, Response Headers & Payload. Convoy records this information for every retry attempt sent. The UI currently shows the last delivery attempt.

### Dead Endpoints
A dead endpoint is an endpoint that failed consecutively to acknowledge events. Currently, we define consecutively failures as at least one event as maxed out it's retry limit to the maximum configured. In the future, we should support different consecutive failure strategies.

### Secrets
Secrets are used to sign the payload when sending events to an endpoint. Creating a secret works as an `upsert` operation. If you don't supply a secret we will generate one for you.

### Hash Functions
Convoy supports the following hash functions - `MD5`, `SHA1`, `SHA224`, `SHA256`, `SHA384`, `SHA512`, `SHA3_224`, `SHA3_256`, `SHA3_384`, `SHA3_512`, `SHA512_224`, `SHA512_256`. Most implementations, however, use - `SHA256` & `SHA512`.

## Release
We adopt a time-based release schedule. Convoy releases a new update every 25th of each month. This is a similar pattern adopted by some open-core companies we like i.e. [Gitlab](https://about.gitlab.com/releases/).
