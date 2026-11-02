# Event payload and search coding-agent task

This is a frozen **uncalibrated task specification**, not an executed agent
benchmark or a product test result. No model attempts or scores are included.
It covers the payload viewer report in issue #2886 and the accompanying explicit
search-mode workflow. The task is intentionally broader than the original report.

Give an agent only `prompt.md` and a fresh checkout at
`001a0e3e0cb78548382e8e95a8e5a7a3a67c79d0`, along with the repository's applicable
instructions. Record its model, configuration, budget, tool access, and final
patch. Do not supply this grading guide, the implementation patch, or evaluator
tests during a controlled attempt. Public history remains a contamination risk.

## Independent grading

Before running agents, freeze an evaluator-owned oracle patch and record its
hash. It must exercise the observable scenarios below without depending on new
helper names or accepting candidate test changes. Apply each candidate's product
patch and the same oracle patch to separate disposable checkouts. Never grade
against a candidate's modified tests. Review changes outside the payload viewer,
event search UI/service/model, event search API/filter logic, documentation, and
necessary tests before admitting a patch.

### Executable oracle for the original payload report

`payload.oracle.spec.ts` imports only classes/modules already present on the
baseline. Its three assertions cover parseable formatter output, rendered JSON,
and clicking a real copy button while collapsed. It stubs clipboard writes;
real clipboard permissions and the additional search UX still require the
integration checks below. Candidate test changes never participate.

Using Python 3.12+, Node, Chrome/Chromium (set `CHROME_BIN` if needed), and trusted
preinstalled dashboard dependencies compatible with the frozen lockfile:

```sh
python3 evals/event-payload-search/grade.py /path/to/candidate --node-modules /path/to/trusted/node_modules --baseline
python3 evals/event-payload-search/grade.py /path/to/candidate --node-modules /path/to/trusted/node_modules
```

These placeholders are evaluator inputs, not repository paths. The runner
archives the frozen baseline into a fresh temporary directory and overlays only
its explicit allowlist of payload viewer, formatter, and reusable copy-button
implementation files. It excludes all product tests and injects the oracle.
Search changes are deliberately not scored by this executable oracle. The
runner does not install dependencies or change the candidate checkout.

Each run retains `run.log` and `summary.json`, including oracle/product hashes,
exit status and every test result. Exactly three successful, non-skipped tests
and a zero process exit are required. Timeouts kill the process group and fail;
missing results, compile errors, and infrastructure errors fail closed. Inspect
logs before sharing because tool output can contain local environment details.
Candidate code executes during grading: use an isolated environment without
personal files, credentials, production access, or network access. Cached
dependencies must be supplied by the evaluator, not by the candidate.

Use these independent checks:

| Oracle | Observable acceptance |
| --- | --- |
| Viewer | Render a nested payload containing quoted keys, escaped strings, booleans, null, and arrays. Parse displayed/copied JSON and compare with the fixture. Collapse the viewer and copy; verify the complete payload is copied. Exercise clipboard failure and keyboard access. |
| Search controls | In a real browser, select metadata, payload, key, and value. Verify key precedes value, the icon/accessible mode name and placeholder/help change, and only one search input is shown. |
| Request contract | Capture actual dashboard requests: metadata sends literal `query` with `queryMode=text`, payload sends normalized `body`, key sends `idempotencyKey`, value sends `idempotencyValue`. Switching modes clears stale search parameters while retaining date/source filters. Reload each resulting URL and verify restored mode/results. |
| Payload validation | `status: "paid"`, `{ "status": "paid" }`, and `data: {status: "paid"}` produce valid JSON objects. Escapes, commas, and key-like text inside strings remain unchanged. Malformed input, empty objects, arrays, and scalars show validation and preserve applied results. |
| Backend compatibility | Literal metadata containing braces stays text with `queryMode=text`; omitted mode retains existing JSON detection; unsupported modes fail validation. Explicit query/body remain an AND filter. |
| Exact lookup | Seed distinct stored keys and raw provider values. Each exact lookup returns its matching event; a prefix does not. Date/source restrictions still apply. Conflicting key/value filters and portal raw-value access are rejected. |
| Running integration | Use a disposable backend with migrations and synthetic events. Verify nesting-sensitive JSON containment, metadata entitlement behavior, both exact lookups, empty results, and the copy flows through the browser. Test mocks alone cannot satisfy this oracle. |

Use synthetic records only: for example payload `{"status":"paid","data":{"status":"pending"}}`,
raw value `provider-delivery-123`, and a distinct stored key produced by the
normal ingestion path. Confirm fixture setup succeeds before attributing an
empty result to search behavior. Do not use production access or credentials.

## Calibration and reporting

Run the fixed oracles on the baseline and then on an immutable reference fix.
Record the reference commit hash once the fix is committed; it is deliberately
not guessed here. Baseline failures must demonstrate incorrect user-visible
behavior, not missing new symbols, compilation errors, unavailable services, or
test setup failures. The reference must pass every oracle, including browser
flows. Retain sanitized commands, toolchain/service versions, exit statuses,
assertions, and browser evidence. Until both runs exist, calibration is pending.

Product unit/regression tests support these oracles but do not constitute agent
attempts. This task includes Angular/browser behavior and cannot be represented
by a Go-only historical test harness. No harness integration is claimed. An
external evaluator must supply browser orchestration and independent assertions
before reporting task passes. Skips, timeouts, missing assertions, and setup
failures are not passes. Report every attempt separately; do not select only
successful runs or infer model quality from one reference implementation.
