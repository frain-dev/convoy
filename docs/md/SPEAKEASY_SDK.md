# Speakeasy public API clients (PDE-755)

Convoy generates public `/api/v1` clients with [Speakeasy](https://www.speakeasy.com/) from `docs/v3/openapi3.yaml`.

## Scope

| Surface | Ownership |
| --- | --- |
| Public API client (JS, Python) | Speakeasy-generated from OpenAPI |
| Webhook signature verify | **Hand-written** in every language (`signature-vectors.json`) |
| Dashboard `/ui` auth/billing/org | Out of scope for PDE-755 |

Follow-up tickets cover Go / Ruby / PHP / Java API codegen.

## Pipeline

```text
@Router annotations → make generate_docs → docs/v3/openapi3.*
        → Speakeasy CI → convoy.js / convoy-python PRs
signature-vectors.json → hand-written verify tests (unchanged)
```

## Required secrets

| Secret | Where | Purpose |
| --- | --- | --- |
| `SPEAKEASY_API_KEY` | `convoy`, `convoy.js`, `convoy-python` | Speakeasy generation |
| `SDK_REPOS_PAT` | `convoy` | Dispatch workflows / open PRs on SDK repos |
| `SDK_BOT_PAT` | `convoy.js`, `convoy-python` | Open generation PRs so verify CI (`run-tests.yml`) triggers — PRs opened with `GITHUB_TOKEN` do not fire `pull_request` workflows. Can be the same fine-grained token as `SDK_REPOS_PAT`. |

## Bootstrap

1. Merge Speakeasy wiring in `convoy`.
2. Add the secrets above.
3. Run **Bootstrap SDK Speakeasy repos** (`bootstrap-sdk-repos.yml`) to open bootstrap PRs on JS + Python.
4. After those merge, OpenAPI changes on `main` trigger `speakeasy-sdk.yml`, which regenerates SDK clients.

## Spec hygiene

- CI workflow `openapi-docs-check.yml` fails if `make generate_docs` dirties committed OpenAPI artifacts.
- Do not annotate `/ui` for this ticket.
