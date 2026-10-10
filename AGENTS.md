# AGENTS.md — awss project instructions

Instructions for AI coding agents working in this repository (Claude Code, GitHub Copilot and any
other tool that reads `AGENTS.md`). It defines what the project does and how to work in it.

This is the single source of truth: `CLAUDE.md` only imports it. Edit this file, not `CLAUDE.md`,
and do not add a tool-specific copy such as `.github/copilot-instructions.md`.

---

## Project overview

**awss** (AWS Search) is a Go CLI tool that searches AWS resources (EC2 instances, ENIs, EBS volumes, VPCs, subnets, S3 buckets and objects, RDS DB instances and clusters, Organization accounts) in parallel
across multiple profiles and regions. It wraps AWS SDK Go v2 and uses Cobra + Viper for CLI wiring.

**Module:** `github.com/dyegoe/awss`
**Go version:** the project follows the latest Go release. The `go` directive in `go.mod` is the
single source: CI and the release builds read it. When Go is upgraded locally, bump the directive
in the same PR (`go mod edit -go=<version>`). Do not hardcode the version in docs.
**Key dependencies:** cobra, viper, aws-sdk-go-v2 (config, ec2, s3, rds, organizations), go-pretty, ini.v1

### Package layout

```text
main.go              — entry point, delegates to cmd.Execute()
cmd/                 — Cobra CLI commands: root (global flags, runSearch/cmdSpec) + one file per resource
search/              — engine registry (engines map, Options) and the parallel profiles × regions fan-out
search/ec2/          — EC2-specific search logic and result type
search/eni/          — ENI-specific search logic and result type
search/ebs/          — EBS volume search logic and result type
search/vpc/          — VPC search logic and result type
search/subnet/       — Subnet search logic, result type, and InCIDRs (overlapping subnets) lookup
search/s3/           — S3 bucket search (per region, client-side name matching)
search/s3obj/        — S3 object (key) search inside given buckets, capped by --max-keys
search/rds/          — RDS DB instance search: AWS filters plus client-side name, version and tag matching
search/rdscluster/   — RDS DB cluster search, reusing the rds local filters; VPC joined from the subnet groups
search/org/          — AWS Organization accounts (one profile, global), and their names for --org-profile
internal/testconv/   — checks the test conventions of every package (run by its own test in CI)
internal/nesting/    — checks the nesting depth of every function (run by its own test in CI)
scripts/             — coverage.awk: the per-package coverage and test-file check of make test
common/              — shared: Results interface, BaseResults, AWS helpers, filter builders,
                       output formatting, reflection row helpers and Version sorting (rows.go), Matcher
                       (match.go), TagMatcher for client-side tag filters (tagmatch.go), utilities
```

The `common.Results` interface is the central contract. Every resource type implements it.
Output rendering (table, JSON, JSON-pretty) is driven entirely by struct tags (`header`, `sort`, `json`, `filter`).

---

## How to work autonomously

Agents are expected to work **fully autonomously** in this repo:

- Read, write, and refactor code without asking for confirmation on individual edits.
- Run `make build` and `make test` after every change, and `make lint` before considering any task
  done.
- Fix any lint errors introduced by your changes before committing.
- Never break existing tests. If a refactor changes a public API, update all call sites and tests.
- Prefer small, focused commits over large sweeping changes — one logical fix per commit.

### Verify commands

```bash
make build
make test   # race detector, random order, every test twice, coverage and test-file check
make lint   # golangci-lint run
```

All three must pass cleanly before any task is considered complete. CI and the pre-commit hook
run the same `make test`, so a green local run means a green CI test step. A shuffle failure
prints its seed: rerun with `go test -shuffle=<seed> ./<package>`.

---

## Standards and guardrails

This section is the single list of rules for this codebase. **MUST** rules block a pull request;
each names what enforces it. A rule marked *review* has no tool that can check it, so the
reviewer and the author own it. **SHOULD** rules may be broken with a reason in the pull request.
`docs/CODESTYLE.md` explains the code rules with examples; it does not add rules.

### MUST: code

| Rule | Enforced by |
|---|---|
| Wrap every error from another module with context (`fmt.Errorf("doing x: %w", err)`); never drop or ignore an error. | `wrapcheck`, `errcheck`, `nilerr` |
| A failing profile or region records its error in its own result set and the run continues; `Search()` never panics or stops the run. | tests (`search` package: logged-out profile, timeout) |
| Nil-check every pointer from an AWS SDK response before use (the SDK's `aws.ToString` for `*string`). | review |
| No `context.TODO()`; pass the caller's `ctx`. `context.Background()` only at the top of `search.Execute` and in helpers with no caller context. | `forbidigo`, `noctx` |
| Control statements nest at most 3 levels deep in a function. | `internal/nesting` (`TestCheck_repository`) |
| Functions stay small: at most 60 lines and 50 statements, cyclomatic complexity at most 15. | `funlen`, `gocyclo` |
| Every exported symbol has a doc comment; an exported function returns exported types. | `revive` |
| Never mutate `Results.Filters` inside `Search()`: one map is shared by every goroutine of a run. Copy it to rewrite it. | test (`TestEngines_searchKeepsFilters`), race detector |
| No hardcoded version string; the version comes from `-ldflags`. | CI version-injection smoke test |
| `//nolint` names the linter and says why, and only where there is no clean fix. | `nolintlint`; review for "no clean fix" |
| Imports stay inside the allowed modules. | `depguard` |
| No AWS call inside a loop over the results of another AWS call: collect IDs, then batch. | review |
| Use the shared helpers of `common` instead of copying reflection, sorting or matching code. | review, `dupl` |
| EC2 `Describe*` calls that do not name IDs send `MaxResults` (see Testing conventions). | tests (fake client page size, per package) |
| `go.mod` and `go.sum` are tidy; code is `gofmt`/`goimports` formatted. | CI tidy diff, golangci-lint formatters |

### MUST: tests

| Rule | Enforced by |
|---|---|
| Every package has tests and covers at least 80% of its statements. | `make test` (`scripts/coverage.awk`) |
| Tests pass with the race detector, in any order, and when run twice. | `make test` (`-race -count=2 -shuffle=on`) |
| Tests follow the naming and structure convention of Testing conventions. | `internal/testconv`, `thelper` |
| No package-level `var` in a test file. | `internal/testconv` |
| Tests never call AWS: fake clients or replaced function variables only. | review |

### MUST: process

| Rule | Enforced by |
|---|---|
| Work on an issue happens on an issue-numbered branch and lands through a pull request; never commit to `main`. | branch protection on `main`, review |
| Commit messages follow Conventional Commits (see Commit message format). | `cz check` (CI `check-commits`, `commit-msg` hook) |
| A pull request title follows Conventional Commits: the squash merge makes it the commit on `main`. | `cz check` (CI `PR title`) |
| Pull requests are squash-merged; agents open them and never merge. | repository settings, review |
| A change to authentication, profiles or shared VPCs carries the exact `awss` commands for the real AWS check in its pull request (see Changes that need a real AWS check). | review |
| A new flag, command, sort field or config key appears in `README.md`. | review |
| A pull request names the issue it closes (`Closes #N`), so merging closes it. | review |
| The `go` directive of `go.mod` is the only Go version; CI and releases read it. | CI (`go-version-file`) |

### SHOULD

- Small, focused commits: one logical change per commit, and every commit builds and passes.
- New packages mock AWS through the SDK's `*APIClient` interfaces (see Testing conventions).
- Extract a helper when a loop body grows beyond about 10 lines.
- Unexported helpers get a doc comment when their purpose is not obvious.
- New test failure messages read `Func(input) = got, want want`.
- CLI sort and filter values are kebab-case (`private-ip`, not `private_ip`).

### Adding a guardrail

A new MUST rule names its enforcement in the same pull request. Before relying on a linter or a
check, prove it fires: add one deliberate violation, see it fail, and remove it. A check that
silently passes is worse than none, because the rule looks enforced.

---

## Issues and workflow

All open work lives in GitHub issues: features, bugs, refactors and small technical improvements.
There is no backlog file; an improvement without an issue gets one first.
Work on an issue happens on an issue-numbered branch (`<number>-short-title`) and lands through a
pull request that closes it (`Closes #N`); never commit straight to `main`. History lives in
`CHANGELOG.md` and git.

- The maintainer reviews and merges pull requests; agents open them and do not merge.
- Pull requests are squash-merged, so release-please lists each change once in the changelog.
  A merge commit would list the same change twice.

### Changes that need a real AWS check

Unit tests never call AWS, so some changes can only be confirmed against real accounts. The
maintainer verifies these on the work setup before merging:

- authentication and profile handling: AWS SSO with Granted, ~150 profiles in
  `~/.awss/config.yaml`, `--profiles all` / `all-profiles`;
- shared VPCs in an AWS Organization: subnets owned by a network account and shared with other
  accounts, VPCs with a secondary CIDR block, instances with several ENIs (`eni`, `ec2 --cidrs`).

For such a change, cover it with unit tests and fake clients, then put the exact `awss` commands
to run and the expected result in the pull request description.

---

## Adding new resource types

When adding a new AWS resource type (e.g. `search/sg/` for Security Groups):

1. Create `search/<resource>/` package with a `Results` struct and `New()` + `Search()` functions.
2. Implement all methods of the `common.Results` interface.
3. Use struct tags `json`, `header`, `sort` on `dataRow` fields — do not add special-case logic to the output layer.
   Implement `GetHeaders`, `GetRows`, `GetSortFields` and `sortResults` with the `common.Headers`,
   `common.Rows`, `common.SortFields` and `common.SortByField` helpers (see `search/ebs/ebs.go`).
4. Add a `filter` struct in `cmd/<resource>.go` with `filter:""` tags matching AWS API filter names.
5. Register the command in `cmd/root.go` via `<resource>InitFlags()` and `<resource>InitViper()`.
6. Register the command in the `engines` map in `search/search.go` (constructor + `GetSortFields`).
7. Wire the `RunE` through `runSearch()` with a `cmdSpec` in the command file (see `cmd/ebs.go` as the pattern).
8. Write tests covering: filter building, result parsing, sort validation, edge cases (nil fields, empty results).
9. Update `README.md` with the new subcommand, its filters, sort fields, and any additional flags.

---

## Testing conventions

- Every package must have a `_test.go` file.
- Test names and structure follow one convention (#173), checked in CI by `internal/testconv`
  (`TestCheck_repository`) and the `thelper` linter. It mirrors the example naming rule of the
  `testing` package and the Go wiki pages TableDrivenTests and CodeReviewComments:
  - `Test<Subject>`, `Test<Type>_<Method>`, plus an optional `_<scenario>` in lowerCamel:
    `TestExecute_timeout`, `TestResults_collect_badSortField`. The subject is an identifier of the
    package, written as declared with its first letter capitalised (`parseSubnet` →
    `TestParseSubnet`; an unexported method stays `TestResults_getFilters`). No `Test_` prefix.
    The test of `main` is `TestMain_<scenario>`: a plain `TestMain` is reserved.
  - Every test function has a doc comment saying what it checks. No commented-out tests.
  - More than one case → a table: `tests := []struct{ name string; … }` with flat fields (no
    gotests `args` struct), `for _, tt := range tests`, `t.Run(tt.name, …)`.
  - Results are `got` and `want`; `t.Fatalf` for a failed precondition, `t.Errorf` for a mismatch.
    A new failure message reads `Func(input) = got, want want`.
  - One `TestResults_accessors` per search package covers `Len` and the `BaseResults` getters.
  - Helpers call `t.Helper()` and take `t` first.
- Mock AWS calls one of two ways, never with real credentials:
  - a package-level function variable that tests replace (`getAwsProfilesFn` in `common/aws.go`,
    `subnetsInCIDRs` in `search/ec2`, `executeSearch` in `cmd`);
  - the SDK's `*APIClient` interfaces, so a fake client drives the paginator (every EC2 search
    package, `search/s3`, `search/s3obj`). Prefer this for new packages: it lets the whole
    `Search()` body be tested.
- EC2 `Describe*` calls that do not name IDs send `MaxResults`: AWS recommends paginated calls
  only, and the SDK paginator does not set a page size by itself (see `pageSize` in `search/eni`).
  This is a precaution: an unpaginated call returned 5250 ENIs from one account without failing.
- `cmd` tests run the real command tree with `rootCmd.ExecuteC()`: start each case with `resetCLI`
  and run it with `runCLI` (`cmd/execute_test.go`). `resetCLI` rebuilds the commands, flags and
  Viper, because a reused pflag slice appends on the next parse. Never use `t.Parallel()` there:
  Cobra and Viper are global.
- Tests must pass in any order and when run again (`go test -count=2 -shuffle=on ./...`). No
  package-level `var` in a test file: return a fresh fixture from a function (`mockResults()` in
  `search/vpc`), since some tests sort or change it in place.
- Output tests live in `common/output_test.go`, with their fixtures in `common/output_data_test.go`.
- Do not make real AWS API calls in tests.

---

## Linter

Config is in `.golangci.yml` (golangci-lint v2 format, `version: "2"`; CI pins the version in
`.github/workflows/common.yml` and the pre-commit hook in `.pre-commit-config.yaml`). The table
in Standards and guardrails says which linter enforces which rule. Formatters (`gofmt`,
`goimports`) are configured in the `formatters` section.

A justified suppression names the linter and the reason:

```go
//nolint:lll // the expected compact JSON is one line by definition
```

---

## Commit message format

```text
<type>(<scope>): <short description>

Types: build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test
Scope: cmd, common, search, search/<resource> (ec2, eni, ebs, vpc, subnet, s3, s3obj, rds, rdscluster, org), release

Reference the issue in the body or title, e.g. "(#82)". `feat` bumps the minor version and `fix`
the patch version on release (release-please), so pick the type by what the user sees.

Examples:
  fix(search/eni): nil-check SubnetId before dereference
  refactor(common): extract BaseResults to eliminate struct duplication
  feat(search/sg): add Security Group resource type (#130)
  test(common): add table-driven tests for FilterTags error path
```
