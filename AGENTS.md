# Working on Convoy

## Start here

- Read [README.md](README.md) and follow [CONTRIBUTING.md](CONTRIBUTING.md).
- Read documentation relevant to the component being changed.
- Verify commands against the current Makefile, scripts, package manifests, and
  CI workflows. Flag discrepancies rather than assuming older examples work.

## Changes

- Keep changes focused on one concern and follow existing package patterns.
- Avoid unrelated refactoring, formatting, and dependency updates.
- Discuss new dependencies and significant API or behavioral changes with
  maintainers before introducing them.
- Preserve existing user changes and update documentation for public changes.
- Review compatibility and migration requirements for API, configuration,
  storage, and event-format changes.

## Go and delivery behavior

- Consult [tiger-style-rules.md](tiger-style-rules.md) and `.golangci.yml`.
- Handle errors explicitly and retain useful diagnostic context.
- Consider concurrency, bounded resource use, cancellation, and timeouts.
- For queues and deliveries, consider retries, duplicates, ordering guarantees,
  and recovery after interruption.
- Preserve authentication, authorization, tenant isolation, and outbound
  destination validation. Never commit secrets or log credentials or tokens.

## Dashboard

- Consult [the dashboard README](web/ui/dashboard/README.md) and package scripts.
- Follow existing frontend conventions.
- Exercise affected browser flows against a running backend.

## Validation and evals

- Add meaningful regression coverage for bug fixes where practical.
- Run formatting, vetting, linting, and tests appropriate to the change.
- `make test` runs Go tests with race detection, excluding E2E packages.
- Select relevant E2E targets from the Makefile and check prerequisites first.
- See [evals/README.md](evals/README.md) for historical coding-agent tasks,
  independent grading, calibration requirements, and privacy rules.
- Existing product tests support grading; running them alone does not measure
  an agent's ability to implement a task. Uncalibrated tasks are candidates.
- For agent comparisons, freeze the task, starting revision, and independent
  grading checks; isolate attempts and record model, duration, and tool failures.
- Report checks actually run, results, and pending validation. Never claim an
  unexecuted check passed or present historical test provenance as an eval run.

## Generated files

- Use established generators rather than manually editing generated output.
- Consult `mockgen`, `generate_docs`, and `docs-generated` Makefile targets.
- Review generated diffs for unintended changes.

## Pull requests and releases

- Explain the problem, resulting behavior, and validation; link relevant issues.
- Use meaningful commit messages and disclose compatibility and operational risks.
- Review outgoing files and descriptions for secrets and local-only information.
- Before release work, read [RELEASE.md](RELEASE.md),
  [the monthly release workflow](docs/monthly-release-workflow.md), and current CI.
- Resolve conflicting release instructions with a maintainer before publishing.
- Distinguish prepared, tested, published, and deployed work.
