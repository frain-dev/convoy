# SDK Speakeasy bootstrap overlays

Files under `convoy.js/` and `convoy-python/` are copied onto the corresponding
SDK repositories by `scripts/sdk/bootstrap-sdk-repos.sh` (also wired as the
**Bootstrap SDK Speakeasy repos** GitHub Action).

Overlays add Speakeasy config, generation workflows, ignore rules for hand-written
verify, migration docs, and README notes. The bootstrap script also bumps package
versions toward the Speakeasy major migration.
