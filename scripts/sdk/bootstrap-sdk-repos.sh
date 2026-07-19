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

  # Remove the deprecated hand-written HTTP client now (intentional 2.x break):
  # persistentEdits preserves any leftovers and they fail compilation under the
  # generated node16 tsconfig (extensionless relative imports). Verify modules
  # stay, protected via .genignore so crypto is never generator-owned.
  rm -rf \
    "${dest}/src/Api" \
    "${dest}/src/client.ts" \
    "${dest}/src/interfaces" \
    "${dest}/src/utils/helpers" \
    "${dest}/tests/convoy.test.ts" \
    "${dest}/tests/routes.test.ts"

  # Minimal entrypoint: keep `require('convoy.js').Webhook` working before and
  # after generation. Explicit .js extensions compile under both the current
  # commonjs tsconfig and Speakeasy's node16 moduleResolution.
  cat > "${dest}/src/convoy.ts" <<'EOF'
/** Speakeasy migration: the hand-written HTTP API was removed for the 2.x
 * break; OpenAPI-generated clients replace it. Webhook verify stays
 * hand-written (see MIGRATION.md). */
export { Webhook } from './webhook.js';
EOF

  # Trim errors to what verify actually uses: the HTTP API exception classes
  # died with the hand-written client, and only they needed http-status.
  cat > "${dest}/src/utils/errors/index.ts" <<'EOF'
class BaseError extends Error {
    public statusCode: number;

    constructor(message?: string, status?: number) {
        super();
        Error.captureStackTrace(this, this.constructor);
        this.name = this.constructor.name;
        this.message = message as string;
        this.statusCode = status as number;
    }
}

class WebhookVerificationException extends BaseError {
    constructor(message: string) {
        super(message);
    }
}

export { WebhookVerificationException };
EOF

  # Keep the protected verify import chain node16-compatible (explicit .js
  # extensions resolve under both commonjs and node16 moduleResolution), and
  # satisfy the generated tsconfig's noUncheckedIndexedAccess: array
  # destructuring yields string | undefined.
  if [[ -f "${dest}/src/webhook.ts" ]]; then
    sed -i "s|from './utils/errors';|from './utils/errors/index.js';|" "${dest}/src/webhook.ts"
    sed -i "s|if (key.trim() === 't') {|if ((key ?? '').trim() === 't') {|" "${dest}/src/webhook.ts"
  fi
  local test_file
  for test_file in "${dest}/tests/webhook.test.ts" "${dest}/tests/shared-vectors.test.ts"; do
    if [[ -f "$test_file" ]]; then
      sed -i "s|from '../src/webhook';|from '../src/webhook.js';|" "$test_file"
    fi
  done

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
# Dead dependencies of the removed hand-written HTTP client; verify needs none
# of them (crypto is node builtin).
for dep in ("@aws-sdk/client-sqs", "axios", "base-64", "http-status", "kafkajs"):
    data.get("dependencies", {}).pop(dep, None)
data.get("devDependencies", {}).pop("@types/base-64", None)
p.write_text(json.dumps(data, indent=4) + "\n")
PY

    # package.json changed (version + removed deps): regenerate the lockfile
    # or `npm ci` fails on the exact-match check.
    if [[ -f "${dest}/package-lock.json" ]]; then
      (cd "$dest" && npm install --package-lock-only --ignore-scripts --no-audit --no-fund)
    fi
  fi

  ensure_readme_note "${dest}/README.md" "$(cat <<'EOF'
## Speakeasy-generated API client

The HTTP API client is generated from Convoy OpenAPI via Speakeasy. **Webhook signature verification remains hand-written** (`src/webhook.ts`) and is covered by shared `tests/signature-vectors.json`. See [MIGRATION.md](./MIGRATION.md).
EOF
)"
}

migrate_convoy_python() {
  local dest="$1"

  # The generated SDK lives at src/convoy/; the old root convoy/ package
  # shadows it (mypy resolves convoy.utils to the old tree and fails on
  # generated imports like FieldMetadata). Move hand-written verify into the
  # generated tree — import path stays `from convoy.utils.webhook import
  # Webhook` — and remove the deprecated hand-written client (1.x break).
  if [[ -f "${dest}/convoy/utils/webhook.py" ]]; then
    mkdir -p "${dest}/src/convoy/utils"
    mv "${dest}/convoy/utils/webhook.py" "${dest}/src/convoy/utils/webhook.py"
  fi
  rm -rf \
    "${dest}/convoy" \
    "${dest}/test/test_client.py" \
    "${dest}/test/test_routes.py"

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

  # Always rebuild the feature branch from the default branch: SDK PRs are
  # squash-merged, so stacking on the stale feature branch makes every
  # follow-up PR conflict with main's squashed copy of the same files.
  # If the remote feature branch exists, capture its SHA as an explicit
  # --force-with-lease expectation (bare lease rejects with "stale info"
  # because the tracking ref from an explicit-refspec fetch is not covered
  # by the single-branch clone's fetch config). Empty expectation means
  # "branch must not exist yet" (first run).
  local expected_sha=""
  if git ls-remote --exit-code --heads origin -- "$BRANCH_NAME" >/dev/null 2>&1; then
    expected_sha="$(git ls-remote origin "refs/heads/${BRANCH_NAME}" | awk '{print $1}')"
  fi
  git checkout -B "$BRANCH_NAME"

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

  git push -u origin "$BRANCH_NAME" --force-with-lease="${BRANCH_NAME}:${expected_sha}"

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
- \`SDK_BOT_PAT\` on this repository so generation PRs trigger verify CI (PRs opened with \`GITHUB_TOKEN\` do not fire \`pull_request\` workflows)

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
