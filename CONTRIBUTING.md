# Contributing

Contributions are welcome, and they are greatly appreciated! Every little bit helps, and credit will always be given. For major changes, please open an issue first to discuss what you would like to change.

You can contribute in many ways:

## Types of Contributions

### Report Bugs

Report bugs at [https://github.com/dyegoe/awss/issues](https://github.com/dyegoe/awss/issues).

If you are reporting a bug, please include:

* Your operating system name and version.
* Any details about your local setup that might be helpful in troubleshooting.
* Detailed steps to reproduce the bug.

### Fix Bugs

Look through the GitHub issues for bugs. Anything tagged with "bug"
is open to whoever wants to implement it.

### Implement Features

Look through the GitHub issues for features. Anything tagged with "enhancement"
is open to whoever wants to implement it. For a smaller technical improvement, open an issue
first, so the work is tracked in one place.

### Submit Feedback

The best way to send feedback is to file an issue at [https://github.com/dyegoe/awss/issues](https://github.com/dyegoe/awss/issues).

If you are proposing a feature:

* Explain in detail how it would work.
* Keep the scope as narrow as possible, to make it easier to implement.
* Remember that this is a volunteer-driven project, and that contributions
  are welcome :)

## Get Started

Ready to contribute? Here's how to set up `awss` for local development.

1. Fork the `awss` repo on GitHub.

2. Clone your fork locally:

    ```bash
    git clone git@github.com:your_name_here/awss.git
    ```

3. Install development tools:

    ```bash
    pip install pre-commit commitizen
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1
    pre-commit install
    ```

    `pre-commit install` installs both the `pre-commit` and `commit-msg` git hooks (see
    `default_install_hook_types` in `.pre-commit-config.yaml`), so commit messages are validated
    against the Conventional Commits format automatically.

4. Create a branch for local development, named after the issue when there is one:

    ```bash
    git checkout -b 123-short-description
    ```

5. Make your changes, following the rules in `AGENTS.md` (Standards and guardrails). Before
   committing, verify everything passes:

    ```bash
    make build
    make test   # the same test command CI runs
    make lint
    ```

6. Commit your changes in the [Conventional Commits](https://www.conventionalcommits.org/)
   format; the types, scopes and how the type drives the release are in `AGENTS.md` (Commit
   message format). Example:

    ```bash
    git add search/ec2/ec2.go search/ec2/ec2_test.go
    git commit -m "fix(search/ec2): nil-check SubnetId before dereference"
    git push origin name-of-your-bugfix-or-feature
    ```

    Or use `make commit` (runs `cz commit`) for an interactive prompt that builds a
    conventional-commit message for you. The `commit-msg` hook and the `check-commits` CI job
    both validate the message format, so malformed messages are rejected before they land.

7. Submit a pull request through the GitHub website. Give it a Conventional Commits title,
    such as `fix(search/ec2): nil-check SubnetId before dereference`: pull requests are
    squash-merged, so the title becomes the commit on `main`, and the `PR title` CI job checks it.

## Pull Request Guidelines

A pull request meets the MUST rules of `AGENTS.md` (Standards and guardrails). Most of them are
checked by `make test`, `make lint` and CI; the ones marked *review* are on you and the reviewer,
so read that table before opening the pull request. A new resource type follows the "Adding new
resource types" checklist in `AGENTS.md`; `docs/CODESTYLE.md` explains the code rules with
examples.

## Releasing (maintainers)

Releases are fully automated by [release-please](https://github.com/googleapis/release-please)
(`.github/workflows/release-please.yml`), driven by the Conventional Commits history —
Commitizen (`.cz.toml`) only enforces that commit messages follow the format; it no longer
computes releases directly.

1. Every push to `main` (i.e. every merged PR, since PRs are squash-merged) updates a standing
   `chore(main): release X.Y.Z` pull request that release-please keeps in sync with `CHANGELOG.md`
   and the next semver version (`feat` → minor, `fix`/`perf` → patch, `!`/`BREAKING CHANGE` →
   major, or minor while pre-1.0.0). Nothing is released while this PR sits open.
2. When you're ready to ship what's accumulated, merge that release PR. Merging it makes
   release-please tag `vX.Y.Z` and publish the GitHub release.
3. Publishing that release triggers `.github/workflows/release.yml`, which builds and attaches
   the linux/darwin amd64/arm64 binaries — no manual build or `gh release create` step needed.
   Publishing a release manually (e.g. backfilling an old tag) triggers the same workflow.

`refs/tags/v*` is protected by a repository ruleset that only admins can bypass, so
`release-please.yml` authenticates with a `RELEASE_PLEASE_TOKEN` PAT (an admin's fine-grained
token scoped to this repo, stored as a repository secret) instead of the default `GITHUB_TOKEN`,
which isn't covered by that bypass.
