# CI layout, race policy, and branch protection

This doc matches the GitHub Actions setup in `.github/workflows/`. Update it when you rename workflows or jobs so admins can keep **required checks** aligned.

## Goals

- **Same tests everywhere:** `pull_request`, `merge_group`, and `push` to `main` all run the race detector (`GO_TEST_RACE=1`) and the full broker E2E matrix when a workflow runs. Nothing is intentionally weaker on PR.
- **Lower time without weaker signal:** parallel `go test` (`TEST_PARALLEL` / `-p` in the Makefile), Go build cache via `actions/setup-go` `cache: true`, less log noise in CI (`GO_TEST_VERBOSE=0`, `TEST_VERBOSE=0`), batched `make test_e2e_fast`, `concurrency` with `cancel-in-progress: true`, and `paths` filters so unrelated edits do not start jobs.

## Race detector and broker suites

| Trigger | Policy |
|--------|--------|
| `pull_request`, `merge_group`, `push` to `main` | **Race on** for `make test`, `make test_e2e_fast`, and `scripts/ci-go-test.sh` broker runs. **No** PR-only skips for SQS or Google Pub/Sub. |

Shared helper: `scripts/ci-go-test.sh` reads `GO_TEST_RACE` and `GO_TEST_VERBOSE` (defaults race on, verbose on if unset; CI sets verbose off explicitly).

## Path filters

Go-heavy workflows use `pull_request.paths` (see each YAML). **Push to `main`** stays unfiltered so post-merge validation still runs the full suite even if a change slipped through a bad filter.

Dashboard-only changes are covered by `.github/workflows/dashboard-build.yml` (`npm ci && npm run build` under `web/ui/dashboard/`).

## Branch protection and required checks

- If you **rename** a workflow file, change its `name:` field, or **split** jobs, GitHub’s required check list may still point at old names. Update repo **Settings → Rules / branch protection** to match the new check identifiers.
- New workflows (for example `dashboard-build.yml`) must be added as required checks if dashboard PRs should be blocked until the build passes.
- When `paths` filters skip a workflow entirely on a PR, that workflow does not appear as a pending check for that PR (by design).

## Local / pre-push verification (suggested)

Before pushing a Go change:

1. `make test` (race on by default).
2. `make test_e2e_fast` when you touch E2E-related code (needs Docker).
3. Dashboard: `cd web/ui/dashboard && npm ci && npm run build` when you touch the dashboard.

Broker pubsub suites (Kafka, SQS, RabbitMQ, Google) run in CI whenever their workflow runs (same commands on PR as on `main`).

## Delivery discipline (single commit)

For larger CI refactors, prefer **one commit** on the PR branch after a green local gate, then **one push**, to avoid burning Actions minutes on intermediate states. Squash or amend per team policy.
