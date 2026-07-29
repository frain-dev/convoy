# Generated reference artifacts

These files are generated from the Convoy source tree so documentation can diff against a stable snapshot.

| File | Generator |
|------|-----------|
| `config-reference.json` | `scripts/docs/genconfigref`: walks `config.Configuration` struct tags (`json`, `envconfig`) and reads defaults from `config.DefaultConfiguration`. |
| `cli-reference.json` | `scripts/docs/gencliref`: builds a Cobra tree (root persistent flags aligned with `cmd/main.go`, plus `server`, `agent`, `migrate`, and `config` subcommands) and exports commands, help strings, and flags. |

## Regenerate

From the repository root:

```bash
make docs-generated
```

Or directly:

```bash
go run ./scripts/docs/genconfigref -output docs/generated/config-reference.json
go run ./scripts/docs/gencliref -output docs/generated/cli-reference.json
```

## Drift checks

After changing `config/config.go`, env tags, defaults, CLI flags in `cmd/main.go`, or Cobra commands under `cmd/server`, `cmd/agent`, `cmd/migrate`, or `cmd/config`, regenerate and commit. In review or CI, run `make docs-generated` and assert a clean `git diff` for `docs/generated/`.

## CI sync (main)

On each push to `main` (and on `workflow_dispatch`), [docs-generated-sync.yml](../../.github/workflows/docs-generated-sync.yml) runs `make docs-generated`, then:

1. If `docs/generated/` drifted on Convoy: `convoy-docs-bot` opens/updates `chore/docs-generated-sync`; `convoy-docs-reviewer` auto-approves only when the diff is under `docs/generated/**`; the bot squash-merges.
2. Mirrors the same artifacts into `frain-dev/convoy-website` at `docs/generated/convoy/` the same way (`chore/sync-convoy-generated-docs`, allowlist `docs/generated/convoy/**`).

No personal access token and no `repository_dispatch`. Secrets live on `frain-dev/convoy`:

| Secret | App |
|--------|-----|
| `DOCS_BOT_APP_ID` / `DOCS_BOT_APP_KEY` | `convoy-docs-bot` |
| `DOCS_REVIEWER_APP_ID` / `DOCS_REVIEWER_APP_KEY` | `convoy-docs-reviewer` |

Install both apps on `frain-dev/convoy` and `frain-dev/convoy-website` (selected repos). Same author/reviewer GitHub App pattern as the cloud staging and SDK bots.

Failure policy: allowlist / author / base misses fail open to human review; merge errors fail the job and leave the PR open.

## Maintaining `gencliref`

Root-level persistent flags are duplicated in `scripts/docs/gencliref/main.go` so the generator does not run `cmd/main.go` (that would run bootstrap hooks). When you add or change global CLI flags, update that block to match `cmd/main.go`.
