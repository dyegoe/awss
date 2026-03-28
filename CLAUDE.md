# CLAUDE.md — awss project instructions

This file is read automatically by Claude Code on every session.
It defines how Claude should behave, what the project does, and how to work in it autonomously.

---

## Project overview

**awss** (AWS Search) is a Go CLI tool that searches AWS resources (EC2 instances, ENIs) in parallel
across multiple profiles and regions. It wraps AWS SDK Go v2 and uses Cobra + Viper for CLI wiring.

**Module:** `github.com/dyegoe/awss`
**Go version:** 1.19 (go.mod) — target 1.21+ on next upgrade
**Key dependencies:** cobra, viper, aws-sdk-go-v2, go-pretty, ini.v1

### Package layout

```text
main.go              — entry point, delegates to cmd.Execute()
cmd/                 — Cobra CLI commands (root, ec2, eni)
search/              — orchestrates parallel search across profiles × regions
search/ec2/          — EC2-specific search logic and result type
search/eni/          — ENI-specific search logic and result type
common/              — shared: interfaces, AWS helpers, output formatting, utilities
```

The `common.Results` interface is the central contract. Every resource type implements it.
Output rendering (table, JSON, JSON-pretty) is driven entirely by struct tags (`header`, `sort`, `json`, `filter`).

---

## How to work autonomously

Claude Code is expected to work **fully autonomously** in this repo:

- Read, write, and refactor code without asking for confirmation on individual edits.
- Run `go build ./...` and `go test ./...` after every change to verify correctness.
- Run `golangci-lint run` before considering any task done.
- Fix any lint errors introduced by your changes before committing.
- Never break existing tests. If a refactor changes a public API, update all call sites and tests.
- Prefer small, focused commits over large sweeping changes — one logical fix per commit.

### Verify commands

```bash
go build ./...
go test ./...
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
- `terminalSize` struct must be exported if `TerminalSize()` is exported (or vice versa — keep consistent).

---

## Fix plan

See `docs/FIXPLAN.md` for the prioritised, ordered list of fixes.
Work through fixes in priority order: P0 → P1 → P2 → P3.
After completing each fix, run the verify commands and check off the item in FIXPLAN.md.

---

## Adding new resource types

When adding a new AWS resource type (e.g. `search/sg/` for Security Groups):

1. Create `search/<resource>/` package with a `Results` struct and `New()` + `Search()` functions.
2. Implement all methods of the `common.Results` interface.
3. Use struct tags `json`, `header`, `sort` on `dataRow` fields — do not add special-case logic to the output layer.
4. Add a `filter` struct in `cmd/<resource>.go` with `filter:""` tags matching AWS API filter names.
5. Register the command in `cmd/root.go` via `<resource>InitFlags()` and `<resource>InitViper()`.
6. Add the new command to the `switch` in `search/search.go` and `getSortFieldsCMDList`.
7. Wire the `RunE` through `runSearch()` in the command file (see `cmd/ebs.go` as the pattern).
8. Write tests covering: filter building, result parsing, sort validation, edge cases (nil fields, empty results).
9. Update `README.md` with the new subcommand, its filters, sort fields, and any additional flags.

---

## Testing conventions

- Every package must have a `_test.go` file.
- Use table-driven tests with named cases (`name string` as first field).
- Mock AWS calls by injecting a function variable (see `getAwsProfiles` in `cmd/root.go` as the pattern).
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

Types: fix, feat, refactor, test, docs, chore
Scope: cmd, search/ec2, search/eni, common, search

Examples:
  fix(search/eni): nil-check SubnetId before dereference
  refactor(common): extract BaseResults to eliminate struct duplication
  feat(search): add Security Group resource type
  test(common): add table-driven tests for FilterTags error path
```
