# Coding-agent evaluations

These tasks measure whether a coding agent can repair historical Convoy bugs
from a frozen pre-fix checkout. They replace the earlier catalog that simply
reran existing product tests. That catalog's results are not agent scores.

`tasks.json` contains 60 **uncalibrated task candidates** selected from public
Git history: a bug-fix commit must add named Go regression tests. Each record
pins the parent revision, reference fix, acceptance scenarios, grading tests,
and permitted implementation paths. Prompts are technical summaries derived
from commit subjects and test names; review them for completeness before use.
They are newly assembled tasks, not historical eval executions.

## What is measured

The agent receives the task prompt, acceptance scenarios, implementation scope,
and a checkout before the fix. It writes a patch. The evaluator applies only
permitted implementation changes to a fresh checkout and adds regression tests
from the historical fix. Candidate test edits are not used for grading.
Every selected test must emit a pass event and its command must exit zero.
Missing tests, skips, timeouts, and compilation failures cannot pass.

This is a functional correctness signal. Passing the selected tests does not
prove that the patch has no regressions or complies with every instruction.
Review scope, compatibility, security, and the agent's verification claims
separately. User-facing tasks also need browser validation against a running
backend before acceptance.

The reference hashes and grader are public for reproducibility. This is an
open-book development suite, not a contamination-resistant model benchmark.
Keep evaluator metadata and fixed revisions out of the agent's supplied context
for controlled runs. A checkout alone cannot prevent the agent accessing public
history; isolation and tool restrictions belong to the external run environment.

## Commands

From the repository root, using Python 3:

```sh
python3 -B evals/harness.py check
python3 -B -m unittest discover -s evals -p 'test_*.py'
python3 -B evals/harness.py list
python3 -B evals/harness.py prepare convoy-agent-001 /tmp/convoy-agent-attempt
```

`prepare` creates a detached worktree and prints the agent prompt and allowed
implementation paths. Give the agent the current public `AGENTS.md` as part of
its instructions: historical checkouts predate that file. Supply a fixed model
and configuration, a time/tool budget, and disposable services. Have the agent
leave edits uncommitted, staging any new files so they are included in the diff.

Grade from the evaluator's current checkout:

```sh
python3 -B evals/harness.py grade convoy-agent-001 /tmp/convoy-agent-attempt
```

Grading creates and removes a separate temporary worktree; it leaves the agent's
checkout intact. Remove that attempt explicitly when no longer needed:

```sh
git worktree remove /tmp/convoy-agent-attempt
```

Use `--force` only after retaining any patch or evidence you need. The harness
does not invoke an agent, call model APIs, or spend model credits. An external
agent runner is needed to produce candidate patches; no provider is assumed.

## Calibrate before reporting scores

For each task, run these commands with the historical toolchain and isolated
service prerequisites:

```sh
python3 -B evals/harness.py baseline convoy-agent-001
python3 -B evals/harness.py reference convoy-agent-001
```

The baseline command intentionally returns a nonzero exit when grading fails.
Inspect the output: an assertion demonstrating the stated bug is evidence;
a missing symbol, compiler failure, skipped test, or missing database is not.
The reference must pass all oracles. Reject or repair tasks whose test patch
cannot apply or whose setup cannot be reproduced. Whole historical test files
can depend on helpers added in the fix; those cases need calibration work.

Store the two runs, environment, commands, and reviewed failure explanation
before marking a candidate calibrated. None of the 60 candidates currently has
this calibration evidence. Do not publish aggregate agent scores until it does.
Repeated agent attempts are needed to assess variability; record each attempt
and the model/configuration rather than selecting only successful runs.

## Services, execution, and privacy

Historical packages can require Postgres, Redis, Docker, migrations, build tags,
and environment settings. Inspect their pinned CI and test setup. Go commands
have a five-minute test timeout and six-minute process timeout per oracle file;
calibrating the full catalog may be expensive. Use disposable services: some
tests clear tables. Grading executes candidate code, so use a sandbox with no
production access, personal files, secrets, or unrestricted credentials.

Use synthetic payloads, identities, domains, tokens, and credentials. Do not
include personal names, email addresses, customer identifiers, private chats,
production payloads, raw traces, or machine paths in fixtures or public results.
Retain only task IDs, revisions, model/configuration identifiers, tool budgets,
duration, outcomes, and sanitized diagnostics needed to explain failures.
Review output before sharing; raw Go output can contain paths or environment
details. There is no production telemetry import or trace upload in this suite.

## CI and maintenance

Normal backend CI validates task references and runs harness unit tests. It
does not call models, grade candidate patches, or claim agent task passes.
Existing product test jobs continue to test product behavior. Model runs and
historical calibration are explicitly initiated work.

`build_tasks.py` deterministically regenerates candidate definitions from a
fixed history anchor. Review generated prompts and scope before admitting a
task to scored runs. Keep real failures as new synthetic regression scenarios;
prioritize observed failure patterns instead of growing the count for its sake.
Do not replace reviewed calibration evidence with regenerated defaults.

Methodology adapted from the general principles in
[A pragmatic guide to LLM evals for devs](https://newsletter.pragmaticengineer.com/p/evals):
ground evaluations in actual failures, define explicit outcomes, and prefer
code-based checks for objective behavior. No article examples or personal data
are reproduced.
