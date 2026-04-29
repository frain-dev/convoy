#!/usr/bin/env bash
set -euo pipefail

JSON="${1:?first arg: path to OSV JSON report}"
OUT_MD="${2:?second arg: path to write Markdown summary}"
OUT_SLACK_DETAILS="${3:-}"

REPO="${GITHUB_REPOSITORY:-convoy}"
RUN_ID="${GITHUB_RUN_ID:-}"
SERVER_URL="${GITHUB_SERVER_URL:-https://github.com}"
if [[ -n "$RUN_ID" ]]; then
  RUN_URL="${SERVER_URL}/${REPO}/actions/runs/${RUN_ID}"
else
  RUN_URL="${SERVER_URL}/${REPO}"
fi
SHA_SHORT="${GITHUB_SHA:-}"
[[ -n "$SHA_SHORT" ]] && SHA_SHORT="${SHA_SHORT:0:7}"

PKG_WITH_VULNS=$(jq '[.results[] | .packages[] | select((.vulnerabilities | length) > 0)] | length' "$JSON")
VULN_ROWS=$(jq '[.results[] | .packages[] | .vulnerabilities[]?] | length' "$JSON")
UNIQUE_IDS=$(jq '[.results[] | .packages[] | .vulnerabilities[]? | .id] | unique | length' "$JSON")

SEV_LINE=$(jq -r '
  [.results[] | .packages[] | .vulnerabilities[]?
    | (.database_specific.severity // "UNKNOWN")
  ] | group_by(.) | map("\(.[0]): \(length)") | join(" · ")
' "$JSON")

read -r CRITICAL HIGH MEDIUM LOW UNKNOWN < <(
  jq -r '
    def normsev:
      ((. // "UNKNOWN") | ascii_upcase) as $s
      | if $s == "MODERATE" then "MEDIUM"
        elif ($s == "CRITICAL" or $s == "HIGH" or $s == "MEDIUM" or $s == "LOW" or $s == "UNKNOWN") then $s
        else "UNKNOWN"
        end;
    [.results[] | .packages[] | .vulnerabilities[]?
      | (.database_specific.severity | normsev)
    ]
    | reduce .[] as $s (
        {"CRITICAL":0,"HIGH":0,"MEDIUM":0,"LOW":0,"UNKNOWN":0};
        .[$s] += 1
      )
    | "\(.CRITICAL) \(.HIGH) \(.MEDIUM) \(.LOW) \(.UNKNOWN)"
  ' "$JSON"
)

BY_ECO=$(jq -r '
  [.results[] | .packages[] | select((.vulnerabilities | length) > 0)
    | .package.ecosystem] | group_by(.) | map("\(.[0]): \(length)") | join(" · ")
' "$JSON")

TOP_PKGS=$(jq -r '
  [.results[] | .packages[] | select((.vulnerabilities | length) > 0)
    | { eco: .package.ecosystem, name: .package.name, ver: .package.version, n: (.vulnerabilities | length) }
  ] | sort_by(-.n) | .[:20]
  | .[] | "| \(.eco) | `\(.name)` | \(.ver) | \(.n) |"
' "$JSON")

ALL_CVE_ROWS=$(jq -r '
  [
    .results[] as $r
    | $r.packages[] as $p
    | select(($p.vulnerabilities | length) > 0)
    | $p.vulnerabilities[] as $v
    | {
        source: ($r.source.path | split("/") | .[-1]),
        ecosystem: $p.package.ecosystem,
        package: $p.package.name,
        version: $p.package.version,
        cve: ($v.id // "UNKNOWN"),
        url: (
          [
            ($v.references // [])[]?
            | .url
            | select((. != null) and (. != "") and (contains(($v.id // ""))))
          ]
          | .[0]
          // (
          [
            ($v.references // [])[]?
            | select((.type // "") == "ADVISORY" or (.type // "") == "WEB")
            | .url
          ]
          | map(select(. != null and . != ""))
          | .[0]
          )
          // ("https://osv.dev/" + ($v.id // ""))
        ),
        severity: (
          (($v.database_specific.severity // "UNKNOWN") | ascii_upcase) as $s
          | if $s == "MODERATE" then "MEDIUM"
            elif ($s == "CRITICAL" or $s == "HIGH" or $s == "MEDIUM" or $s == "LOW" or $s == "UNKNOWN") then $s
            else "UNKNOWN"
            end
        ),
        fixed: (
          [
            ($v.affected // [])[]?
            | .ranges[]?
            | .events[]?
            | .fixed?
          ]
          | map(select(. != null and . != ""))
          | .[0] // "--"
        ),
        aliases: (($v.aliases // []) | join(",")),
        summary: (
          ($v.summary // ($v.details // ""))
          | gsub("\\r|\\n"; " ")
          | gsub("\\|"; "/")
          | if length > 110 then .[0:110] + "..." else . end
        )
      }
  ]
  | sort_by(.severity, .ecosystem, .package, .cve)
  | .[]
  | "\(.severity)\t\(.ecosystem)\t\(.package)\t\(.version)\t\(.fixed)\t\(.cve)\t\(.url)\t\(.aliases)\t\(.summary)\t\(.source)"
' "$JSON")

DETAILED_ROWS_MD=$(jq -r '
  [
    .results[] as $r
    | $r.packages[] as $p
    | select(($p.vulnerabilities | length) > 0)
    | $p.vulnerabilities[] as $v
    | {
        severity: (
          (($v.database_specific.severity // "UNKNOWN") | ascii_upcase) as $s
          | if $s == "MODERATE" then "MEDIUM"
            elif ($s == "CRITICAL" or $s == "HIGH" or $s == "MEDIUM" or $s == "LOW" or $s == "UNKNOWN") then $s
            else "UNKNOWN"
            end
        ),
        ecosystem: $p.package.ecosystem,
        package: $p.package.name,
        version: $p.package.version,
        fixed: (
          [
            ($v.affected // [])[]?
            | .ranges[]?
            | .events[]?
            | .fixed?
          ]
          | map(select(. != null and . != ""))
          | .[0] // "--"
        ),
        advisory: ($v.id // "UNKNOWN"),
        url: (
          [
            ($v.references // [])[]?
            | .url
            | select((. != null) and (. != "") and (contains(($v.id // ""))))
          ]
          | .[0]
          // (
          [
            ($v.references // [])[]?
            | select((.type // "") == "ADVISORY" or (.type // "") == "WEB")
            | .url
          ]
          | map(select(. != null and . != ""))
          | .[0]
          )
          // ("https://osv.dev/" + ($v.id // ""))
        ),
        aliases: (($v.aliases // []) | join(", ")),
        summary: (
          ($v.summary // ($v.details // ""))
          | gsub("\\r|\\n"; " ")
          | gsub("\\|"; "/")
          | if length > 160 then .[0:160] + "..." else . end
        )
      }
  ]
  | sort_by(.severity, .ecosystem, .package, .advisory)
  | .[]
  | "| \(.severity) | \(.ecosystem) | `\(.package)` | \(.version) | \(.fixed) | `\(.advisory)` | [link](\(.url)) | \(.aliases) | \(.summary) |"
' "$JSON")

SOURCES=$(jq -r '.results[] | "- `\(.source.path | split("/") | .[-1])`"' "$JSON")

{
  echo "# OSV vulnerability report"
  echo
  echo "**Repository:** \`${REPO}\`  "
  echo "**Commit:** \`${SHA_SHORT:-n/a}\`  "
  echo "**Workflow run:** ${RUN_URL}"
  echo
  echo "## Summary"
  echo
  echo "| Metric | Value |"
  echo "|--------|-------|"
  echo "| Packages with ≥1 finding | ${PKG_WITH_VULNS} |"
  echo "| Vulnerability records (rows) | ${VULN_ROWS} |"
  echo "| Unique OSV IDs | ${UNIQUE_IDS} |"
  echo
  echo "**By ecosystem (affected packages):** ${BY_ECO:-—}"
  echo
  echo "**By advisory severity (row labels):** ${SEV_LINE:-—}"
  echo
  echo "## Scanned sources"
  echo
  echo "${SOURCES}"
  echo
  echo "## Top packages by finding count"
  echo
  echo "| Ecosystem | Package | Version | # findings |"
  echo "|-----------|---------|---------|------------|"
  if [[ -n "$TOP_PKGS" ]]; then
    echo "$TOP_PKGS"
  else
    echo "| — | — | — | 0 |"
  fi
  echo
  echo "## Detailed advisories (fix + details)"
  echo
  echo "| Severity | Ecosystem | Package | Version | Fixed | Advisory | URL | Aliases | Summary |"
  echo "|----------|-----------|---------|---------|-------|----------|-----|---------|---------|"
  if [[ -n "$DETAILED_ROWS_MD" ]]; then
    echo "$DETAILED_ROWS_MD"
  else
    echo "| — | — | — | — | — | — | — | — | none |"
  fi
  echo
  echo "---"
  echo "_Generated by OSV-Scanner nightly workflow._"
} >"$OUT_MD"

if [[ -n "$OUT_SLACK_DETAILS" ]]; then
  {
    echo "OSV nightly detailed CVE report"
    echo "Repo: ${REPO} | Commit: ${SHA_SHORT:-n/a}"
    echo "Run: ${RUN_URL}"
    echo
    echo "Summary"
    echo "Packages with findings: ${PKG_WITH_VULNS}"
    echo "Vulnerability rows: ${VULN_ROWS}"
    echo "Unique OSV IDs: ${UNIQUE_IDS}"
    echo "By ecosystem: ${BY_ECO:-N/A}"
    echo "Severity totals -> *CRITICAL:* ${CRITICAL}  *HIGH:* ${HIGH}  *MEDIUM:* ${MEDIUM}  *LOW:* ${LOW}  *UNKNOWN:* ${UNKNOWN}"
    echo
    echo "ALL CVEs:"
    echo
    if [[ -n "$ALL_CVE_ROWS" ]]; then
      while IFS=$'\t' read -r sev eco pkg ver fixed cve url aliases summary src; do
        case "$sev" in
          CRITICAL) icon="🔴" ;;
          HIGH) icon="🟠" ;;
          MEDIUM) icon="🟡" ;;
          LOW) icon="🟢" ;;
          *) icon="⚪" ;;
        esac
        printf "%s *%s* · <%s|%s>\n" "$icon" "$sev" "$url" "$cve"
        printf "• Package: %s %s@%s\n" "$eco" "$pkg" "$ver"
        printf "• Fixed: %s\n" "$fixed"
        if [[ -n "$aliases" ]]; then
          printf "• Aliases: %s\n" "$aliases"
        fi
        if [[ -n "$summary" ]]; then
          printf "• Summary: %s\n" "$summary"
        fi
        printf "• Source: %s\n" "$src"
        printf "—\n"
      done <<<"$ALL_CVE_ROWS" || true
    else
      echo "- none"
    fi
  } >"$OUT_SLACK_DETAILS"
fi
