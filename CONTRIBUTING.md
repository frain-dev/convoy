# How to contribute

We definitely welcome your patches and contributions to convoy!

If you are new to github, please start by
reading [Pull Request howto](https://help.github.com/articles/about-pull-requests/)

## Guidelines for Pull Requests

How to get your contributions merged smoothly and quickly.

- Create **small PRs** that are narrowly focused on **addressing a single concern**. We often times receive PRs that are
  trying to fix several things at a time, but only one fix is considered acceptable, nothing gets merged and both
  author's & review's time is wasted. Create more PRs to address different concerns and everyone will be happy.

- The convoy package should only depend on standard Go packages and a small number of exceptions. If your contribution
  introduces new dependencies which are NOT in the [list](https://pkg.go.dev/github.com/frain-dev/convoy?tab=imports),
  you need a discussion with the convoy creators.

- For speculative changes, consider opening an issue and discussing it first. If you are suggesting a behavioral or API
  change, consider starting with a proposal draft.

- Provide a good **PR description** as a record of **what** change is being made and **why** it was made. Link to a
  github issue if it exists.

- Don't fix code style and formatting unless you are already changing that line to address an issue. PRs with irrelevant
  changes won't be merged. If you do want to fix formatting or style, do that in a separate PR.

- Unless your PR is trivial, you should expect there will be reviewer comments that you'll need to address before
  merging. We expect you to be reasonably responsive to those comments, otherwise the PR will be closed after 2-3 weeks
  of inactivity.

- Maintain **clean commit history** and use **meaningful commit messages**. PRs with messy commit history are difficult
  to review and won't be merged. Use
  `rebase -i upstream/main` to curate your commit history and/or to bring in latest changes from main (but avoid
  rebasing in the middle of a code review).

- Keep your PR up to date with upstream/main (if there are merge conflicts, we can't really merge your change).

- **All tests need to be passing** before your change can be merged. We recommend you **run tests locally** before
  creating your PR to catch breakages early on.
    - `make all` to test everything, OR
    - `make vet` to catch vet errors
    - `make test` to run the tests
    - `make testrace` to run tests in race mode

- For convenience, run `make setup` to run set relevant configurations, like a pre-commit hook to regenerate openapi docs on every commit to files in the server folder.
- Exceptions to the rules can be made if there's a compelling reason for doing so.

## Guidelines for Raising Issues

When raising issues, it's important to provide as much context and detail as possible. This helps maintainers and other contributors understand the issue and makes it easier for them to address it. Here's a template you can follow when creating a new issue:

- **Title**: Provide a clear and concise title that summarizes the issue.
- **Description**: Provide a detailed description of the issue. Include any relevant information that can help others understand the issue.
- **Steps to Reproduce**: Provide detailed steps to reproduce the issue. This should be a list of actions that lead to the issue occurring.
- **Expected Results**: Describe what you expected to happen when you followed the steps to reproduce.
- **Actual Results**: Describe what actually happened when you followed the steps to reproduce.
- **Error Messages/Logs**: If applicable, include any error messages or logs that are related to the issue.