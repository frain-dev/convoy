# Release Process Plan

## Goals
- Enforce Conventional Commit-style PR titles with scopes `controlplane`, `dataplane`, `dashboard`.
- On every merge to `main`, build binaries + Docker images and publish them (Cloudsmith + DockerHub) **without** tags or GitHub releases.
- Stable releases use CalVer tags in the form `v{yy}.{month}.{patch}` with GitHub release name matching the tag (no prefix).
- At end of each month, cut a tag and publish a stable release for customers.
- Use GoReleaser and add git-cliff for changelogs with sections: Features, Improvements, Bug Fixes.

## Current State (quick)
- `release.yml` runs on tag `v*` and uses GoReleaser config in `.publisher.yml`.
- `build-image.yml` runs on tag `v*` and pushes DockerHub images.
- `build-rolling-image.yml` runs on `main` but targets AWS ECR, not DockerHub.
- GoReleaser release name is `{{ .ProjectName }}-v{{ .Version }}`.
- No PR title validation workflow yet.

## Proposed Flow (high level)
1. **PR title enforcement** on all PRs.
2. **Main branch publish** (every merge to `main`):
   - GoReleaser publishes binaries to Cloudsmith (no GitHub release, no tag).
   - Docker images push to DockerHub using `main-<shortsha>` and `sha-<shortsha>` tags.
3. **Monthly stable release**:
   - A scheduled workflow runs `git-cliff` and opens a release PR with updated `CHANGELOG.md`.
   - After PR merge, a tag `vX.Y.Z` is created (manual workflow_dispatch or automated).
   - Tag triggers existing release workflows (binaries + DockerHub).

## Exact Code Changes (file-by-file)

### 1) Add PR title validation
**New file:** `.github/workflows/conventional-pr-title.yml`
- Trigger: `pull_request` (opened, edited, reopened, synchronize).
- Action: `amannn/action-semantic-pull-request@v6`
- Enforce:
  - `types`: `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `chore`, `ci`, `build`
  - `scopes`: `controlplane`, `dataplane`, `dashboard`
  - `requireScope: true`
  - `subjectPattern`: `.+` (non-empty)
- Example allowed PR title: `feat(controlplane): add webhook retry policy`

### 2) Main branch publish (no tags/releases)
**New file:** `.github/workflows/release-main.yml`
- Trigger: `push` on `main`.
- Steps (mirrors existing `release.yml` structure):
  - Build UI and upload artifact.
  - Download UI artifact.
  - Run GoReleaser with a **main-only config** that disables GitHub releases and uses snapshot versioning.

**New file:** `.publisher-main.yml`
- Copy `.publisher.yml` and change:
  - `release.disable: true` (skip GitHub releases entirely)
  - `snapshot: true` (allow tagless build)
  - Optional: `snapshot.name_template: "{{ .ShortCommit }}"` to set deterministic version string
- Keep publishers (Cloudsmith) enabled so binaries still publish.

**Workflow GoReleaser step:**
- `goreleaser release --clean -f .publisher-main.yml`

### 3) Main branch DockerHub images
**New file:** `.github/workflows/build-image-main.yml`
- Trigger: `push` on `main`.
- Steps:
  - Build UI artifact (same as `build-image.yml`).
  - Build/push `amd64` and `arm64` images to DockerHub with tags:
    - `getconvoy/convoy:main-<shortsha>`
    - `getconvoy/convoy:sha-<shortsha>`
  - Create multi-arch manifest for `main-<shortsha>` and `sha-<shortsha>`.
- This keeps tag-based `build-image.yml` intact for stable releases.

### 4) Stable tag release naming
**Update:** `.publisher.yml`
- Change `release.name_template` to:
  - `name_template: "v{{ .Version }}"`
- Keep `draft: false` (ensures no draft releases on tag).

### 5) Monthly release PR (git-cliff)
**New file:** `.git-cliff.toml`
- Configure changelog sections:
  - Features -> `feat`
  - Improvements -> `perf`, `refactor`, `chore`, `build`, `ci`
  - Bug Fixes -> `fix`
- Example output format:
  - `## Features`
  - `## Improvements`
  - `## Bug Fixes`

**New file:** `.github/workflows/monthly-release-pr.yml`
- Trigger: `schedule` (e.g. last day of month at 02:00 UTC).
- Steps:
  - Checkout `main`.
  - Run `git-cliff` to update `CHANGELOG.md`.
  - Open or update a PR via `peter-evans/create-pull-request@v6`.
  - PR title: `chore(release): prepare monthly release`
  - PR body includes changelog summary.
  - This workflow only prepares the release PR; it does **not** tag or publish.

### 6) Tag stable release (end of month, CalVer)
**Option A (manual, safer):**
**New file:** `.github/workflows/tag-stable-release.yml`
- Trigger: `workflow_dispatch` with inputs: `yy`, `month`, `patch` (or `version` string).
- Steps:
  - Create annotated tag `v{yy}.{month}.{patch}` on `main`.
  - Push tag.
- Tag triggers existing `release.yml` and `build-image.yml`.
- This workflow is **independent** of `monthly-release-pr.yml` (it can be run anytime).
- In practice, you would run it **after** the monthly PR is merged to ensure the changelog is up to date.

**Option B (fully automated):**
- Same workflow runs on schedule **after** release PR is merged (or checks that it has been merged).
- Uses latest version in `CHANGELOG.md` or computes next tag based on input.

## Notes / Decisions Needed
- GoReleaser docs confirm `release.disable: true` disables SCM release creation, while custom `publishers` still run (so Cloudsmith publishing remains possible without a GitHub release).
- Monthly tagging choice: **Option B (automated)**, using the latest version from `CHANGELOG.md`.
- DockerHub tag scheme for main builds: `main-<sha>` and `sha-<sha>`.

## Rollout Order
1. Add PR title check.
2. Add main publish workflows (GoReleaser + DockerHub).
3. Update `.publisher.yml` release name template.
4. Add git-cliff config + monthly PR workflow.
5. Add stable tag workflow.
