# AGENTS.md — awss project instructions

Instructions for AI coding agents working in this repository (Claude Code, GitHub Copilot and any
other tool that reads `AGENTS.md`). It defines what the project does and how to work in it.

This is the single source of truth: `CLAUDE.md` only imports it. Edit this file, not `CLAUDE.md`,
and do not add a tool-specific copy such as `.github/copilot-instructions.md`.

---

## Project overview

**awss** (AWS Search) is a Go CLI tool that searches AWS resources (EC2 instances, ENIs, EBS volumes, VPCs, subnets, S3 buckets and objects) in parallel
across multiple profiles and regions. It wraps AWS SDK Go v2 and uses Cobra + Viper for CLI wiring.

**Module:** `github.com/dyegoe/awss`
**Go version:** the project follows the latest Go release. The `go` directive in `go.mod` is the
single source: CI and the release builds read it. When Go is upgraded locally, bump the directive
in the same PR (`go mod edit -go=<version>`). Do not hardcode the version in docs.
**Key dependencies:** cobra, viper, aws-sdk-go-v2 (ec2, s3, sts), go-pretty, ini.v1

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
common/              — shared: Results interface, BaseResults, AWS helpers, filter builders,
                       output formatting, reflection row helpers (rows.go), Matcher (match.go), utilities
```

The `common.Results` interface is the central contract. Every resource type implements it.
Output rendering (table, JSON, JSON-pretty) is driven entirely by struct tags (`header`, `sort`, `json`, `filter`).

---

## How to work autonomously

Agents are expected to work **fully autonomously** in this repo:

- Read, write, and refactor code without asking for confirmation on individual edits.
- Run `go build ./...` and `go test -race ./...` after every change to verify correctness.
- Run `golangci-lint run` before considering any task done.
- Fix any lint errors introduced by your changes before committing.
- Never break existing tests. If a refactor changes a public API, update all call sites and tests.
- Prefer small, focused commits over large sweeping changes — one logical fix per commit.

### Verify commands

```bash
go build ./...
go test -race ./...
golangci-lint run
```

All three must pass cleanly before any task is considered complete.

---

## Code style

See `docs/CODESTYLE.md` for the full style guide.

Summary of non-negotiable rules:

- No naked `if err != nil { return }` that silently swallows errors — always wrap with context.
- No `context.TODO()` — use `context.Background()` at call sites or accept and pass `ctx context.Context`.
- No hardcoded version strings — version must be injected via `-ldflags`.
- Nil-check all pointer dereferences from AWS SDK responses before use.
- Max nesting depth: 3 levels. Extract early-return guards or helper functions to reduce nesting.
- All exported symbols must have a doc comment.
- Never mutate `Results.Filters` inside `Search()`: the map is shared by every goroutine of a run. Copy it.

---

## Backlog and workflow

Open improvements live in `docs/BACKLOG.md`; feature requests live in GitHub issues.
Work on an issue happens on an issue-numbered branch (`<number>-short-title`) and lands through a
pull request; never commit straight to `main`. When an item is done, remove it from the backlog
(history lives in `CHANGELOG.md` and git), do not tick it.

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
- Use table-driven tests with named cases (`name string` as first field).
- Mock AWS calls one of two ways, never with real credentials:
  - a package-level function variable that tests replace (`getAwsProfilesFn` in `common/aws.go`,
    `subnetsInCIDRs` in `search/ec2`);
  - the SDK's `*APIClient` interfaces, so a fake client drives the paginator (every EC2 search
    package, `search/s3`, `search/s3obj`). Prefer this for new packages: it lets the whole
    `Search()` body be tested.
- EC2 `Describe*` calls that do not name IDs send `MaxResults`: AWS recommends paginated calls
  only, and the SDK paginator does not set a page size by itself (see `pageSize` in `search/eni`).
  This is a precaution: an unpaginated call returned 5250 ENIs from one account without failing.
- Test files for output live in `common/output_test.go` — use `output_test_data.go` for fixtures.
- Do not make real AWS API calls in tests.
- Target ≥ 80% coverage per package.

---

## Linter

Config is in `.golangci.yml` using **golangci-lint v2** format (`version: "2"`).
Enabled linters include: `errcheck`, `govet`, `gosec`, `misspell`,
`funlen` (60 lines / 50 statements), `gocyclo` (max 15), `dupl`, `lll`, `noctx`.
Formatters (`gofmt`, `goimports`) are configured in the `formatters` section (v2 convention).

`//nolint:<linter>` comments are allowed only when there is no clean fix and the suppression has a
comment explaining why. Example:

```go
for _, inst := range i.Instances { //nolint:gocritic // rangeValCopy: AWS SDK struct is not pointer-based
```

---

## Commit message format

```text
<type>(<scope>): <short description>

Types: build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test
Scope: cmd, common, search, search/<resource> (ec2, eni, ebs, vpc, subnet, s3, s3obj), release

Reference the issue in the body or title, e.g. "(#82)". `feat` bumps the minor version and `fix`
the patch version on release (release-please), so pick the type by what the user sees.

Examples:
  fix(search/eni): nil-check SubnetId before dereference
  refactor(common): extract BaseResults to eliminate struct duplication
  feat(search/sg): add Security Group resource type (#130)
  test(common): add table-driven tests for FilterTags error path
```
