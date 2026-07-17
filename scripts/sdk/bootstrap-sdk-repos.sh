#!/usr/bin/env bash
# Bootstrap Speakeasy generation config into convoy.js and convoy-python and open PRs.
#
# Required env:
#   SDK_REPOS_PAT  — token with contents:write + pull_requests:write on both SDK repos
# Optional:
#   BRANCH_NAME    — feature branch name (default: feat/speakeasy-bootstrap-pde-755)
#   DRY_RUN        — if "1", write local clones only and do not push/open PRs

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BOOTSTRAP_DIR="${ROOT_DIR}/sdk/bootstrap"
BRANCH_NAME="${BRANCH_NAME:-feat/speakeasy-bootstrap-pde-755}"
DRY_RUN="${DRY_RUN:-0}"
WORK_DIR="${WORK_DIR:-$(mktemp -d)}"

# Bootstrap must land as a reviewable feature branch, never a protected ref;
# SDK_REPOS_PAT could otherwise force-push straight to main on the SDK repos.
case "$BRANCH_NAME" in
  main|master|release/*|-*|*[!a-zA-Z0-9._/-]*)
    echo "Refusing to bootstrap onto branch '$BRANCH_NAME' (protected or invalid name)" >&2
    exit 1
    ;;
esac

if [[ -z "${SDK_REPOS_PAT:-}" && "$DRY_RUN" != "1" ]]; then
  echo "SDK_REPOS_PAT is required (unless DRY_RUN=1)" >&2
  exit 1
fi

export GH_TOKEN="${SDK_REPOS_PAT:-${GH_TOKEN:-}}"

# Authenticate git via gh's credential helper instead of embedding the token
# in remote URLs (which would persist in .git/config and leak in error output).
# --force: configure even when the host only has env-token (GH_TOKEN) auth.
if [[ -n "$GH_TOKEN" ]]; then
  gh auth setup-git --hostname github.com --force
fi

ensure_readme_note() {
  local readme="$1"
  local note="$2"
  if ! grep -q "Speakeasy-generated API client" "$readme" 2>/dev/null; then
    printf '\n%s\n' "$note" >> "$readme"
  fi
}

migrate_convoy_js() {
  local dest="$1"

  # Hand-written API stays in place until the first Speakeasy generation PR replaces
  # it as an intentional convoy.js 2.x break. Verify modules are protected via
  # .genignore so crypto is never generator-owned.

  if [[ -f "${dest}/src/convoy.ts" ]] && ! grep -q "Speakeasy migration" "${dest}/src/convoy.ts"; then
    sed -i '1s|^|/** Speakeasy migration: hand-written HTTP API is deprecated; next major replaces this with OpenAPI-generated clients. Webhook verify stays hand-written. */\n|' "${dest}/src/convoy.ts"
  fi

  if [[ -f "${dest}/package.json" ]]; then
    python3 - <<'PY'
import json
from pathlib import Path
p = Path("package.json")
data = json.loads(p.read_text())
data["version"] = "2.0.0-alpha.0"
data["description"] = "Convoy JS SDK (Speakeasy-generated API client; hand-written webhook verify)"
scripts = data.setdefault("scripts", {})
scripts["test:verify"] = "jest --config jestconfig.json --testPathPattern='(webhook|shared-vectors)'"
p.write_text(json.dumps(data, indent=4) + "\n")
PY
  fi

  ensure_readme_note "${dest}/README.md" "$(cat <<'EOF'
## Speakeasy-generated API client

The HTTP API client is generated from Convoy OpenAPI via Speakeasy. **Webhook signature verification remains hand-written** (`src/webhook.ts`) and is covered by shared `tests/signature-vectors.json`. See [MIGRATION.md](./MIGRATION.md).
EOF
)"
}

migrate_convoy_python() {
  local dest="$1"

  if [[ -f "${dest}/convoy/convoy.py" ]] && ! grep -q "Speakeasy migration" "${dest}/convoy/convoy.py"; then
    sed -i '1s|^|"""Speakeasy migration: hand-written HTTP API is deprecated; next major replaces this with OpenAPI-generated clients. Webhook verify stays hand-written."""\n|' "${dest}/convoy/convoy.py"
  fi

  if [[ -f "${dest}/setup.py" ]]; then
    python3 - <<'PY'
from pathlib import Path
import re
text = Path("setup.py").read_text()
text = re.sub(r'version="[^"]+"', 'version="1.0.0a0"', text, count=1)
text = re.sub(
    r'description="[^"]+"',
    'description="Python SDK for Convoy (Speakeasy-generated API client; hand-written webhook verify)"',
    text,
    count=1,
)
Path("setup.py").write_text(text)
PY
  fi

  ensure_readme_note "${dest}/README.md" "$(cat <<'EOF'
## Speakeasy-generated API client

The HTTP API client is generated from Convoy OpenAPI via Speakeasy. **Webhook signature verification remains hand-written** (`convoy/utils/webhook.py`) and is covered by shared `test/signature-vectors.json`. See [MIGRATION.md](./MIGRATION.md).
EOF
)"
}

clone_and_apply() {
  local repo="$1"
  local src_dir="${BOOTSTRAP_DIR}/${repo}"
  local dest="${WORK_DIR}/${repo}"

  echo "==> Bootstrapping frain-dev/${repo}"
  rm -rf "$dest"

  git clone --depth 1 "https://github.com/frain-dev/${repo}.git" "$dest"

  cd "$dest"

  # Re-runs must track the remote feature branch so --force-with-lease has a
  # local expected ref. Shallow clone only fetches the default branch.
  if git ls-remote --exit-code --heads origin -- "$BRANCH_NAME" >/dev/null 2>&1; then
    git fetch --depth 1 origin "refs/heads/${BRANCH_NAME}:refs/remotes/origin/${BRANCH_NAME}"
    git checkout -B "$BRANCH_NAME" "origin/${BRANCH_NAME}"
  else
    git checkout -B "$BRANCH_NAME"
  fi

  # Prefer rsync when available; fall back to cp for local dry-runs.
  if command -v rsync >/dev/null 2>&1; then
    rsync -a --exclude '.git' "${src_dir}/" "${dest}/"
  else
    cp -a "${src_dir}/." "${dest}/"
  fi

  case "$repo" in
    convoy.js) migrate_convoy_js "$dest" ;;
    convoy-python) migrate_convoy_python "$dest" ;;
    *) echo "Unknown repo: $repo" >&2; exit 1 ;;
  esac

  git add -A
  if git diff --cached --quiet; then
    echo "No changes for ${repo}; skipping"
    return 0
  fi

  git -c user.name="convoy-bot" -c user.email="engineering@getconvoy.io" \
    commit -m "$(cat <<EOF
feat: bootstrap Speakeasy API client generation (PDE-755)

Wire Speakeasy generation from convoy docs/v3/openapi3.yaml.
Keep webhook signature verify hand-written and covered by shared
signature-vectors.json. First Speakeasy generation ships as a new
major so hand-written API shapes are not silently broken.
EOF
)"

  if [[ "$DRY_RUN" == "1" ]]; then
    echo "DRY_RUN=1: leaving changes in ${dest}"
    git --no-pager log -1 --oneline
    git --no-pager diff --stat HEAD~1 || true
    return 0
  fi

  git push -u origin "$BRANCH_NAME" --force-with-lease

  local pr_url
  pr_url="$(gh pr list --repo "frain-dev/${repo}" --head "$BRANCH_NAME" --json url --jq '.[0].url' || true)"
  if [[ -z "$pr_url" || "$pr_url" == "null" ]]; then
    pr_url="$(gh pr create \
      --repo "frain-dev/${repo}" \
      --base main \
      --head "$BRANCH_NAME" \
      --title "feat: Speakeasy API client bootstrap (PDE-755)" \
      --body "$(cat <<EOF
## Summary
- Bootstrap Speakeasy generation from \`frain-dev/convoy\` \`docs/v3/openapi3.yaml\`
- Keep webhook signature verification **hand-written** (shared \`signature-vectors.json\`)
- Bump toward a new major so the first Speakeasy generation is an intentional API-shape break
- Add CI that regenerates via Speakeasy and still runs verify vector tests

## Secrets required
- \`SPEAKEASY_API_KEY\` on this repository (Actions)

## Follow-up
After merge, run **SDK Generation** (\`sdk_generation.yaml\`) with \`force=true\` to open the first generated API-client PR.

Related: PDE-755 / frain-dev/convoy Speakeasy workflow.
EOF
)"
)"
  fi

  echo "Opened/updated PR for ${repo}: ${pr_url}"
}

clone_and_apply "convoy.js"
clone_and_apply "convoy-python"

echo "Done. Work dir: ${WORK_DIR}"
