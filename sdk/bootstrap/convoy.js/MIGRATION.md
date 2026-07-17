# convoy.js 2.x migration (Speakeasy)

## What changed

- The **public HTTP API client** will be generated from Convoy's OpenAPI spec (`docs/v3/openapi3.yaml`) via [Speakeasy](https://www.speakeasy.com/).
- **Webhook signature verification stays hand-written.** Generators do not own crypto. `src/webhook.ts` and the shared `tests/signature-vectors.json` contract remain the source of truth for verify (see `.genignore`).

## Breaking change policy

Shipping the Speakeasy client is an intentional **2.x** break. Hand-written `1.x` method shapes under `src/Api/**` are **not** silently adapted to look the same.

1. This bootstrap PR wires Speakeasy + protects verify.
2. The first `sdk_generation.yaml` run opens a PR that replaces the hand-written HTTP client with generated code and publishes as `2.x`.
3. Consumers pin `1.x` until they migrate call sites.

## Verify (unchanged)

```js
const { Webhook } = require('convoy.js');

const webhook = new Webhook({
  header: request.headers['x-convoy-signature'],
  payload: rawBody,
  secret: endpointSecret,
});

webhook.verify();
```

## Regenerating the API client

CI on `frain-dev/convoy` triggers Speakeasy when OpenAPI artifacts change. Locally (requires `SPEAKEASY_API_KEY`):

```bash
speakeasy run
```
