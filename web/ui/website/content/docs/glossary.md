---
title: Glossary
description: 'Glossary'
id: glossary
order: 8
---


# Glossary

This section collects brief definitions of some of the technical terms used in the documentation for Convoy.


## Groups

Groups are used to create logical contexts or separate environments (dev, staging & production). Different groups can also be created for different teams each with their own login details on the same convoy deployment.

## Applications

An application represents a user's application trying to receive webhooks. Once you create an application on Convoy. You receive an `app_id` that you should save and supply in subsequent API calls to perform other actions E.g. Send an event. Currently, an application maps to one endpoint. In the future, an application should map to multiple endpoints. When you creating an application, you should supply a [secret](#secrets).

## Endpoints

An endpoint represents a target URL to receive events. An endpoint can be in either of these states - `active`, `inactive` or `pending`. When an endpoint is in the `inactive` state all events sent will be saved but not dispatched until the endpoint is re-enabled. These events are known set to the `Discarded` state.

## Events

An event represents a specific event triggered by your system. Convoy persists events sent to dead endpoints with a status - `Discarded`. This enables users re-activate their endpoints and easily retry events without the need to re-trigger the events from your systems.

## Event Types

Events are sent to an endpoint depending on the event type, which is defined when creating the endpoint defaulting to `"*"` if not set, which is a catch all for all events. An endpoint can define multiple event types, as such it will receive an event from all those events. Event types are matched using direct string comparison and are case sensitive. Support for regex event matching is planned.

## Delivery Attempts

A delivery attempt represents a single attempt to dispatch an event to an endpoint. Specifically, it contains 2 things - Request Headers & Payload, Response Headers & Payload. Convoy records this information for every retry attempt sent. The UI currently shows only the last delivery attempt. The number of delivery attempts and retry strategy can be configured per group.

## Dead Endpoints

A dead endpoint is an endpoint that failed consecutively to acknowledge events. Currently, we define consecutively failures as at least one event as maxed out it's retry limit to the maximum configured. In the future, we should support different consecutive failure strategies.

## Secrets

Secrets are used to sign the payload when sending events to an endpoint. If you don't supply a secret convoy will generate one for you.

## Hash Functions

We have found out that most implementations use - `SHA256` & `SHA512`. However, convoy also supports the following hash functions:
- `MD5`
- `SHA1` 
- `SHA224`
- `SHA256`
- `SHA384`
- `SHA512`
- `SHA3_224`
- `SHA3_256`
- `SHA3_384`
- `SHA3_512`
- `SHA512_224`
- `SHA512_256` 

## Replay Attacks

A replay attack occurs when an attacker intercepts a valid network payload with the intent of fraudulently re-transmitting the payload. Convoy supports replay attack prevention by including a timestamp in the request header under the key `Convoy-Timestamp`. This timestamp is also included in the signature-header and is signed together with the request body using the endpoint secret. Therefore, an attacker cannot change the timestamp without invalidating the signature. Take the following steps to verify your signature and prevent replay attacks;

1. Extract the timestamp and the signed signature-header from the request header, extract the request body.
2. Prepare a string by concatenating the timestamp followed by a `,` and the request body.
3. Generate a signature of the concatenated string using the endpoint secret and your hashing algorithm (e.g `SHA256`)
4. Compare the newly generated signature with the value in the signature-header, if the signatures match, check the time interval between the timestamp and the current time. In your system, set a tolerance on this time interval to prevent replay attacks.

## Releases

We adopt a time-based release schedule.  A new release is created on the 25th of every month, over the course of the month we ship patches and bug fixes for that release. This is a similar pattern adopted by some open-core companies we like i.e. [Gitlab](https://about.gitlab.com/releases/). Convoy adopts [SemVar v2.0.0](https://semver.org/spec/v2.0.0.html).

## Rate Limiting Endpoints

While you are guaranteed you'll be able to receive events as fast as possible using convoy, your customers might not be able to handle events coming to their systems at the same rate which might cause a disruption of service on their end.  You can control the number events you want to send to an application's endpoint by setting a rate limit and a rate limit duration on each endpoint. The default is `5000` in `1m` i.e. 5,000 requests per minute.

## Retry Schedule

When an application's endpoint is experiencing temporary disruption of service, events sent to them might fail requiring you to retry them. Convoy allow you to set the number of attempts to a particular endpoint and how to initiate the retry. Convoy supports two retry strategies
- `default`: retries are done in linear time. It's best to set a reasonable number of attempts if the duration is short.
- `exponential-backoff`:  retries events while progressively increasing the time before the next attempt. The default schedule looks like this:
	-	10 seconds
	-	30 seconds
	-	1 minute
	-	3 minutes
	-	5 minutes
	-	10 minutes
	-	15 minutes
Retry strategies are configured per group