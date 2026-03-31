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

On each push to `main`, the workflow [docs-generated-sync.yml](../../.github/workflows/docs-generated-sync.yml) runs `make docs-generated`. If `docs/generated/` changes, it opens or updates pull request branch `chore/docs-generated-sync` against `main` (only files under `docs/generated/` are included). That job uses the default `GITHUB_TOKEN` with `contents` and `pull-requests` write permission.

### Notify convoy-website (cross-repo)

A second job (`needs: sync`) runs only after the sync job succeeds. It sends `repository_dispatch` to `${GITHUB_REPOSITORY_OWNER}/convoy-website` with `event_type` `convoy_docs_sync` and `client_payload.convoy_ref` set to the push commit SHA (`github.sha`), so the website repo can check out that ref.

**Repository secret (required on the Convoy server repo):** `CONVOY_WEBSITE_DISPATCH_TOKEN`: a personal access token (classic: `repo` scope, or fine-grained: **Contents** write on `convoy-website`) allowed to call the GitHub API and create a repository dispatch on the target repo. Add it under **Settings → Secrets and variables → Actions** on this repository.

If the sync job fails, the dispatch job is skipped. If the secret is missing or the token lacks access to `convoy-website`, the dispatch job fails after sync completes successfully.

## Maintaining `gencliref`

Root-level persistent flags are duplicated in `scripts/docs/gencliref/main.go` so the generator does not run `cmd/main.go` (that would run bootstrap hooks). When you add or change global CLI flags, update that block to match `cmd/main.go`.
