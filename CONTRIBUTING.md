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

Versioning follows [Semantic Versioning](https://semver.org/) and is derived from the
Conventional Commits history via [Commitizen](https://commitizen-tools.github.io/commitizen/)
(`.cz.toml`, `version_provider = "scm"` reads the version straight from git tags).

1. On `main`, with a clean working tree, run:

    ```bash
    make release
    ```

    This runs `cz bump --changelog`, which inspects commits since the last tag, picks the next
    semver version (`feat` → minor, `fix`/`perf` → patch, `!`/`BREAKING CHANGE` → major, or minor
    while `major_version_zero` is `0.x`), regenerates `CHANGELOG.md`, and creates an annotated
    `vX.Y.Z` tag.

2. Push the changelog commit and the tag:

    ```bash
    git push origin main --follow-tags
    ```

3. Create the GitHub release (this triggers `.github/workflows/release.yml`, which builds and
   attaches the linux/darwin amd64/arm64 binaries):

    ```bash
    gh release create vX.Y.Z --notes-file <(cz changelog vX.Y.Z --dry-run)
    ```
