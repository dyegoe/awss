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

Look through the GitHub issues for features. Anything tagged with "feature"
is open to whoever wants to implement it.

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

4. Create a branch for local development:

    ```bash
    git checkout -b name-of-your-bugfix-or-feature
    ```

5. Make your changes. Before committing, verify everything passes:

    ```bash
    make build
    make test
    make lint
    ```

    Or equivalently:

    ```bash
    go build ./...
    go test ./...
    golangci-lint run
    ```

6. Commit your changes following the [Conventional Commits](https://www.conventionalcommits.org/) format:

    ```text
    <type>(<scope>): <short description>

    Types: build, bump, chore, ci, docs, feat, fix, perf, refactor, revert, style, test
    Scope: cmd, search/ec2, search/eni, common, search
    ```

    Example:

    ```bash
    git add search/ec2/ec2.go search/ec2/ec2_test.go
    git commit -m "fix(search/ec2): nil-check SubnetId before dereference"
    git push origin name-of-your-bugfix-or-feature
    ```

    Or use `make commit` (runs `cz commit`) for an interactive prompt that builds a
    conventional-commit message for you. The `commit-msg` hook and the `check-commits` CI job
    both validate the message format, so malformed messages are rejected before they land.

7. Submit a pull request through the GitHub website.

## Pull Request Guidelines

Before you submit a pull request, check that it meets these guidelines:

1. All three verify commands must pass: `make build`, `make test`, `make lint`.
2. If the pull request adds functionality, update the docs and add tests.
3. New exported symbols must have doc comments (see `docs/CODESTYLE.md`).
4. Follow the coding conventions described in `docs/CODESTYLE.md`.

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
3. That tag/release triggers `.github/workflows/build-binaries.yml`, which builds and attaches
   the linux/darwin amd64/arm64 binaries — no manual build or `gh release create` step needed.

Publishing a release manually (e.g. backfilling an old tag) still works: publishing any GitHub
release triggers `.github/workflows/release.yml`, which calls the same binary build.
