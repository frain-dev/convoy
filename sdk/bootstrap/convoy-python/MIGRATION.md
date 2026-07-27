# convoy-python 1.x migration (Speakeasy)

## What changed

- The **public HTTP API client** will be generated from Convoy's OpenAPI spec (`docs/v3/openapi3.yaml`) via [Speakeasy](https://www.speakeasy.com/).
- **Webhook signature verification stays hand-written.** Generators do not own crypto. `src/convoy/utils/webhook.py` and the shared `test/signature-vectors.json` contract remain the source of truth for verify (see `.genignore`).

## Breaking change policy

Shipping the Speakeasy client is an intentional **1.x** break from the hand-written `0.x` surfaces. Method shapes are **not** silently preserved.

1. This bootstrap PR wires Speakeasy, removes the deprecated hand-written HTTP client, and relocates verify to `src/convoy/utils/webhook.py` (inside the generated module tree, so `from convoy.utils.webhook import Webhook` keeps resolving — `moduleName: convoy` in `.speakeasy/gen.yaml`).
2. The first `sdk_generation.yaml` run opens a PR that adds the OpenAPI-generated client and publishes as `1.x`.
3. Consumers pin `0.x` until they migrate call sites.

## Verify (unchanged)

```python
from convoy.utils.webhook import Webhook

webhook = Webhook(secret="endpoint-secret")
payload = request.body.decode("utf-8")
signature = request.headers.get("X-Convoy-Signature", "")

if not webhook.verify_signature(payload, signature):
    raise PermissionError("invalid signature")
```

## Regenerating the API client

CI on `frain-dev/convoy` triggers Speakeasy when OpenAPI artifacts change. Locally (requires `SPEAKEASY_API_KEY`):

```bash
speakeasy run
```
