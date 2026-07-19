# Public API clients (PDE-755)

Convoy generates public `/api/v1` clients from `docs/v3/openapi3.yaml`. One
pattern, per-language generator picks:

| Repo | Generator | Why |
| --- | --- | --- |
| `convoy.js` | [Speakeasy](https://www.speakeasy.com/) (free tier) | Free tier allows exactly one generated SDK per workspace; JS holds the slot |
| `convoy-python` | [openapi-python-client](https://github.com/openapi-generators/openapi-python-client) (OSS, pinned) | Speakeasy slot taken; OSS output proven idiomatic and complete. Speakeasy pipeline kept dormant in-repo for a provider switch |
| `convoy-java` | [OpenAPI Generator](https://openapi-generator.tech/) (OSS, pinned; `java`/`native` library) | Same OSS pattern as Python; native `java.net.http` + Jackson, no framework deps |

## Scope

| Surface | Ownership |
| --- | --- |
| Public API client (JS, Python, Java) | Generated from OpenAPI |
| Webhook signature verify | **Hand-written** in every language (`signature-vectors.json`) |
| Dashboard `/ui` auth/billing/org | Out of scope for PDE-755 |

Follow-up tickets cover Go / Ruby / PHP API codegen (Go candidate:
`oapi-codegen`; Ruby/PHP deferred until demand signal).

## Pipeline

```text
@Router annotations → make generate_docs → docs/v3/openapi3.*
        → speakeasy-sdk.yml (dispatcher) → sdk_generation.yaml on each SDK repo
              convoy.js:      Speakeasy action → regen PR
              convoy-python:  openapi-python-client → regen PR (only on diff)
              convoy-java:    OpenAPI Generator → regen PR (only on diff)
signature-vectors.json → hand-written verify tests (unchanged)
```

The dispatcher only contracts on the workflow filename (`sdk_generation.yaml`)
and inputs (`force`, `feature_branch`); each repo owns its generator.

## Spec fidelity rules (learned the hard way)

- `json.RawMessage` / `[]byte` fields need explicit `swaggertype` annotations
  or they leak into the spec as integer arrays (convoy#2736).
- A bare `{type: object}` is a CLOSED object: strict generators (zod) strip
  every payload key. `docs/fix_openapi_spec.sh` opens bare-object properties
  with `additionalProperties: true` at doc-gen time (convoy#2737).

## Required secrets

| Secret | Where | Purpose |
| --- | --- | --- |
| `SPEAKEASY_API_KEY` | `convoy`, `convoy.js` | Speakeasy generation (JS only now) |
| `SDK_REPOS_PAT` | `convoy` | Dispatch workflows / open PRs on SDK repos |
| `SDK_BOT_PAT` | `convoy.js`, `convoy-python`, `convoy-java` | Open generation PRs so verify CI triggers — PRs opened with `GITHUB_TOKEN` do not fire `pull_request` workflows. Can be the same fine-grained token as `SDK_REPOS_PAT` (which must also cover `convoy-java`). |

## Speakeasy plan limit

The free tier allows **one generated SDK per workspace** (`smarts-org`);
convoy.js consumed it. Python's Speakeasy generation hit
`generation access blocked` (2026-07-19) and was replaced with
openapi-python-client the same day. If a trial/discount lands (Sagar email /
Subomi ask), switch Python back by swapping the Speakeasy job (preserved in
the dormant, manual-only `speakeasy_generation.yaml`) back into
`sdk_generation.yaml`. The dispatcher always targets the
`sdk_generation.yaml` filename on the SDK repos, so that file must keep
existing under that name whichever generator it runs.

## Spec hygiene

- CI workflow `openapi-docs-check.yml` fails if `make generate_docs` dirties committed OpenAPI artifacts.
- Do not annotate `/ui` for this ticket.
