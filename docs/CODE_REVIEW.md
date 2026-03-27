# Code Review: awss

Snapshot taken 2026-03-27 after the FIXPLAN pass.

---

## Scorecard

| Area                    | Score | Notes                                                       |
| ----------------------- | :---: | ----------------------------------------------------------- |
| Project structure       |   A   | Clean package layout, single responsibility per package     |
| Error handling          |  B+   | Good after P0-2 fix; `cmd/root.go` helpers still bare       |
| Test coverage           |  C+   | common 89%, ec2 48%, eni 29%, cmd 23%, search 18%           |
| API design (interfaces) |  A-   | `Results` interface is solid; `BaseResults` embedding clean |
| Dependency hygiene      |   A   | 9 direct deps, all well-known, no bloat                     |
| Lint discipline         |   A   | 0 warnings, 14 justified nolint directives                  |
| Concurrency             |   B   | Goroutine fan-out works; no cancellation/timeout wiring     |
| Documentation           |   B   | Exported symbols documented; no package-level README yet    |
| CI / release pipeline   |  B-   | Basic; no caching, no matrix Go versions, no e2e smoke      |

**Overall: B+** -- solid CLI tool with good bones, held back mainly by test gaps and some architectural debt in the `common` package.

---

## Strengths

1. **Clear package contract.** The `common.Results` interface is the spine of the project. Adding a new resource type is mechanical -- just implement the interface. This is the most important design decision in the codebase.

2. **Lean dependency tree.** 9 direct deps for a CLI that talks to AWS, renders tables, and reads config files is tight. No unnecessary frameworks.

3. **Reflection-driven output.** Using struct tags (`header`, `sort`, `json`, `filter`) to drive rendering and filtering avoids large switch statements and keeps the data types as the single source of truth.

4. **Parallel search.** Profiles x regions fan-out with goroutines and channels is the right pattern for this workload.

5. **Lint hygiene.** Zero lint warnings. Every `nolint` has a justification comment. This signals discipline.

---

## Areas to Improve

### 1. Test coverage is the biggest gap

```text
common   89%   -- good
ec2      48%   -- moderate
eni      29%   -- low
cmd      23%   -- low
search   18%   -- low
```

**Impact:** The `Search()` methods, which are the core business logic, have zero test coverage (all test stubs are commented out). A bug in response parsing would not be caught.

**Suggested approach:**

- Inject the AWS client as an interface (e.g. `ec2iface.DescribeInstancesAPI`) so you can pass a mock in tests.
- Start with table-driven tests for `parseInstance()` and `parseENIRow()` -- these are pure functions that take SDK types and return dataRows. Easy wins.
- For `cmd/`, use Cobra's `ExecuteC()` in tests with captured stdout.

### 2. The `common` package does too many things

It currently holds: AWS helpers, string utilities, tag parsing, output rendering, the `Results` interface, and `BaseResults`. This makes it a dependency magnet -- every package imports it.

**Suggested split (when it feels right):**

- `awsutil/` -- `AwsConfig`, `WhoAmI`, `GetAwsProfiles`, filter builders
- `output/` -- `PrintResults`, table/json renderers, `Bold`
- `results/` -- `Results` interface, `BaseResults`
- `common/` -- generic helpers (`StringValue`, `StringInSlice`, etc.)

This is not urgent but would reduce coupling as you add more resource types.

### 3. No cancellation or timeout on AWS calls

`context.Background()` is passed everywhere. If one region hangs, the whole run hangs. Wiring a `context.WithTimeout` at the top of `search.Execute()` would be a small change with large reliability impact.

### 4. `WhoAmI` pre-auth is a workaround that could bite

```go
// Workaround to avoid to spam Okta with too many requests.
if runOnce {
    if _, err := common.WhoAmI(profile, region); err != nil {
        return err
    }
    runOnce = false
}
```

This only pre-auths the first profile. If you use multiple profiles with different Okta configs, the remaining profiles still hit Okta in parallel. Consider pre-authing all unique profiles sequentially before the fan-out.

### 5. Reflection is used heavily for simple operations

`GetHeaders()`, `GetRows()`, `sortResults()`, and `StructToFilters()` all use `reflect`. This works but:

- It's slow (not an issue at this scale, but worth knowing).
- Errors surface at runtime, not compile time.
- A typo in a struct tag silently produces wrong behavior.

Consider generating these with `go generate` or switching to explicit methods if the number of resource types stays small (< 5).

### 6. No integration / smoke test in CI

The CI runs unit tests but never builds the binary and runs `awss ec2 --help` or similar. A single smoke test catches linking errors and flag registration bugs that unit tests miss.

---

## Growth Roadmap

Ordered by impact-to-effort ratio:

| Priority | Item                                        | Effort | Impact |
| -------- | ------------------------------------------- | ------ | ------ |
| 1        | Add tests for `parseInstance`/`parseENIRow` | Small  | High   |
| 2        | Wire `context.WithTimeout` in `Execute()`   | Small  | High   |
| 3        | Add CI smoke test (`awss ec2 --help`)       | Small  | Medium |
| 4        | Inject AWS client interface for testability | Medium | High   |
| 5        | Split `common` package                      | Medium | Medium |
| 6        | Add a new resource type (e.g. SG)           | Medium | Medium |
| 7        | Pre-auth all profiles before fan-out        | Small  | Low    |
| 8        | Replace reflection with codegen             | Large  | Low    |

---

## Code Metrics Snapshot

```text
Production code:  1,911 lines across 6 packages
Test code:        2,428 lines (1.27x ratio -- good)
Direct deps:      9
nolint directives: 14 (all justified)
go vet warnings:  0
golangci-lint:    0 issues
```
