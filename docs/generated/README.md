# Generated reference artifacts

These files are produced from the Convoy source tree so documentation can diff against a stable snapshot.

| File | Generator |
|------|-----------|
| `config-reference.json` | `scripts/docs/genconfigref` — walks `config.Configuration` struct tags (`json`, `envconfig`) and reads defaults from `config.DefaultConfiguration`. |
| `cli-reference.json` | `scripts/docs/gencliref` — builds a Cobra tree (root persistent flags aligned with `cmd/main.go`, plus `server`, `agent`, `migrate`, and `config` subcommands) and exports commands, help strings, and flags. |

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

After changing `config/config.go`, env tags, defaults, CLI flags in `cmd/main.go`, or Cobra commands under `cmd/server`, `cmd/agent`, `cmd/migrate`, or `cmd/config`, regenerate and commit. In review or CI you can run `make docs-generated` and assert a clean `git diff` for `docs/generated/`.

## Maintaining `gencliref`

The root-level persistent flags are duplicated in `scripts/docs/gencliref/main.go` so the generator does not execute `cmd/main.go` (which would run bootstrap hooks). When you add or change global CLI flags, update that block to match `cmd/main.go`.
