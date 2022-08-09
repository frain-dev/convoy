# Releases

This page describes the release process for convoy.

## How to cut an Individual release

These instruction is currently only valid for this repo.

### Branch management and versioning strategy

We use [Semantic Versioning](https://semver.org/).

We maintain a separate branch for each minor release, named `release-<major>.<minor>`, e.g. `release-1.1`, `release-2.0`.

Note that branch protection kicks in automatically for any branches whose name starts with `release-`. Never use names starting with `release-` for branches that are not release branches.

- Every new feature and change should be merged into the main branch with the exception of bug fixes. This can be managed in two different ways: 
   - Bug fixes that need to exist on the release branches should be merged to main and backported to the release branch.
   - Bug fixes that need not exist on main should be merged to the release branch only. 
- During a release, the release manager will ensure no PR is forgotten in the release branch.
- In principle, we treat the main branch and release branches as divergent branches. All commits to main need not exist on the release branch and vice versa. The process above is to help to implement this principle. In cases, where it is not as straightforward to resolve, the release manager will direct based on their best sense of judgement.

Maintaining the release branches for older minor releases happens on a best effort basis.

### 0. Updating dependencies

A few days before a major or minor release, consider updating the dependencies.

Then create a pull request against the main branch.

Note that after a dependency update, you should look out for any weirdness that
might have happened. Such weirdnesses include but are not limited to: flaky
tests, differences in resource usage, panic.

In case of doubt or issues that can't be solved in a reasonable amount of time,
you can skip the dependency update or only update select dependencies. In such a
case, you have to create an issue or pull request in the GitHub project for
later follow-up.

#### Updating Go dependencies

TBD.

#### Updating Angular dependencies

TBD.

### 1. Prepare your release

At the start of a new major or minor release cycle create the corresponding release branch based on the main branch. For example if we're releasing `2.17.0` and the previous stable release is `2.16.0` we need to create a `release-2.17` branch. Note that all releases are handled in protected release branches, see the above `Branch management and versioning` section. Release candidates and patch releases for any given major or minor release happen in the same `release-<major>.<minor>` branch. Do not create `release-<version>` for patch or release candidate releases.

Changes for a patch release or release candidate should be merged into the previously mentioned release branch via pull request.

Bump the version in the `VERSION` file and update `CHANGELOG.md`. Do this in a proper PR pointing to the release branch as this gives others the opportunity to chime in on the release in general and on the addition to the changelog in particular. For a release candidate, append something like `-rc.0` to the version (with the corresponding changes to the tag name, the release name etc.).

Note that `CHANGELOG.md` should only document changes relevant to users of Prometheus, including external API changes, performance improvements, and new features. Do not document changes of internal interfaces, code refactorings and clean-ups, changes to the build process, etc. People interested in these are asked to refer to the git history.

For release candidates still update `CHANGELOG.md`, but when you cut the final release later, merge all the changes from the pre-releases into the one final update.

Entries in the `CHANGELOG.md` are meant to be in this order:

-   `[CHANGE]`
-   `[FEATURE]`
-   `[ENHANCEMENT]`
-   `[BUGFIX]`

### 2. Draft the new release

Tag the new release via the following commands:

```bash
$ tag="v$(< VERSION)"
$ git tag -s "${tag}" -m "${tag}"
$ git push origin "${tag}"
```

Optionally, you can use this handy `.gitconfig` alias.

```ini
[alias]
  tag-release = "!f() { tag=v${1:-$(cat VERSION)} ; git tag -s ${tag} -m ${tag} && git push origin ${tag}; }; f"
```

Then release with `git tag-release`.

Once a tag is created, the release process through Github actions will take care of the rest.

TODO: A missing step here which should be later automated. A release needs to be created before the assets can be uploaded to match the tag. :)

Finally, wait for the build step for the tag to finish. The point here is to wait for tarballs to be uploaded to the Github release and the container images to be pushed to the Docker Hub and Quay.io. Once that has happened, click _Publish release_, which will make the release publicly visible and create a GitHub notification.

## Release Manager
The release manager is the individual responsible for the release. The following are the responsibilities of the release manager: 

- Decides what goes into the release and what doesn't go into the release using their best sense of judgement. He/She should ensure that features well tested and only well tested features are released.
- Before every release minor or major release, ensure all commits have been properly cherry-picked or backported.
- Ensure Changelog is up to date in a user readable format. That is, Changelog should be written with the end-user perspective in mind, not blindly copying commit messages. 
- Ensure website is up to date with release documentation. 
