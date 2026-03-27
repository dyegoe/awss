# Fix Plan — awss

Ordered list of improvements to reach an A+ codebase.
Claude Code works through this list top-to-bottom, checking off items as they are completed.
After each fix: run `go build ./...`, `go test ./...`, `golangci-lint run` — all must pass.

Legend: [ ] to do · [x] done · [~] in progress

---

## P0 — Crash / correctness bugs (fix first)

- [x] **P0-1: Nil dereference in `search/eni/eni.go`**
  - `*eni.SubnetId` is dereferenced unconditionally. Detached ENIs may return nil.
  - Fix: replace with `common.StringValue(eni.SubnetId)`.
  - Also audit every other `*ptr` dereference in `eni.go` and `ec2.go` for the same issue.
  - Test: add a test case with a nil SubnetId in the mock response.

- [x] **P0-2: `FilterTags` silently swallows parse errors (`common/aws.go`)**
  - Current: `if err != nil { return filters }` — caller gets empty filters, no error.
  - Fix: change signature to `func FilterTags(tags []string) ([]types.Filter, error)` and propagate the error.
  - Update all callers: `cmd/ec2.go`, `cmd/eni.go`, and any tests.
  - Test: add a test case for malformed tag input (e.g. `"NoEqualsSign"`).

---

## P1 — Code quality and idioms (high impact)

- [x] **P1-1: Replace `context.TODO()` with `context.Background()`**
  - Files: `search/ec2/ec2.go`, `search/eni/eni.go`, `common/aws.go`.
  - Fix: global replace `context.TODO()` → `context.Background()`.
  - Note: `context.Background()` is the correct production value; `TODO` signals unfinished work.
  - Do not yet refactor `Search()` to accept a `ctx` parameter — that is P2-3.

- [x] **P1-2: Inject version via ldflags (`cmd/root.go`)**
  - Current: `Version: "0.7.3", // TODO: Remember to update...`
  - Fix:
    1. Add `var version = "dev"` as a package-level var in `cmd/root.go`.
    2. Change the command to `Version: version`.
    3. Update `.github/workflows/release.yml` to pass `-ldflags="-X github.com/dyegoe/awss/cmd.version=${{ github.ref_name }}"` to the build step.
    4. Update `Makefile` (if present) or add a build note in README.

- [x] **P1-3: Fix `terminalSize` export inconsistency (`common/common.go`)**
  - Current: unexported type `terminalSize` returned by exported `TerminalSize()`.
  - Fix option A (preferred): rename `TerminalSize()` → `getTerminalSize()` since it is only used internally in `common/output.go`.
  - Fix option B: export the type as `TerminalSize` and rename the function to `GetTerminalSize()`.
  - Choose option A unless external callers need the type.

- [x] **P1-4: Fix typo in struct comments**
  - `search/ec2/ec2.go` line: `// Errors contains the erros found during the search.`
  - `search/eni/eni.go` line: same typo.
  - Fix: `erros` → `errors` in both files.

- [x] **P1-5: Reduce nesting in `search/eni/eni.go` `Search()` function**
  - The inner loop has 4+ nesting levels due to `if attachment != nil` guards and IP loops.
  - Fix: extract a helper `func (r *Results) parseENIRow(eni types.NetworkInterface) (dataRow, error)`.
  - Apply early-return guard clauses inside the helper.
  - This also makes the function comply with `funlen` (≤60 lines).

- [x] **P1-6: Reduce nesting in `search/ec2/ec2.go` `Search()` function**
  - The Reservations → Instances double loop with ENI inner loop can be flattened.
  - Fix: extract `func (r *Results) parseInstance(inst types.Instance) dataRow`.
  - The ENI slice building inside the instance loop should be a one-liner using a helper.

---

## P2 — Architecture improvements

- [x] **P2-1: Extract `common.BaseResults` to eliminate struct duplication**
  - `ec2.Results` and `eni.Results` have identical fields (`Profile`, `Region`, `Errors`, `SortField`)
    and identical getter methods (`GetProfile`, `GetRegion`, `GetErrors`, `GetSortField`).
  - Fix:
    1. Create `common/results.go` with `BaseResults` struct and its four getter methods.
    2. Embed `common.BaseResults` in `ec2.Results` and `eni.Results`.
    3. Delete the duplicated getter implementations in both packages.
    4. Verify `common.Results` interface is still satisfied by running `go build ./...`.

- [x] **P2-2: Fix N+1 API calls in `search/eni/eni.go`**
  - Current: `SearchInstanceName()` is called once per ENI inside the response loop — N separate `DescribeInstances` API calls.
  - Fix:
    1. Collect all non-nil `eni.Attachment.InstanceId` values into a slice after the main loop.
    2. If the slice is non-empty, make a single `DescribeInstances` call with all IDs.
    3. Build a `map[string]string` of instanceID → instanceName from the response.
    4. In a second pass over the rows, populate `InstanceName` from the map.
  - Also consider: remove `searchEC2.SearchInstanceName()` from `search/ec2/ec2.go` if it is no longer needed externally, or keep it for other callers.

- [x] **P2-3: Add `ctx context.Context` parameter to `Search()` (interface update)**
  - Update `common.Results` interface: `Search(ctx context.Context)`.
  - Update `ec2.Results.Search()`, `eni.Results.Search()`.
  - Update `search/search.go` to pass a context from `Execute()`.
  - Pass `context.Background()` at the top of `Execute()` for now; wire cancellation later.
  - Update all tests that call `Search()`.

- [ ] **P2-4: Split `common` package into sub-concerns**
  - `common/aws.go` imports AWS SDK types and is tightly coupled to AWS.
  - `common/common.go` and `common/output.go` are generic utilities.
  - Proposed split:
    - `common/aws.go` → stays as is (AWS-specific helpers and filters).
    - `common/util.go` → rename from `common.go` (StringValue, StringInSlice, etc.).
    - `common/output.go` → stays.
    - `common/results.go` → new file for BaseResults and the Results interface (from P2-1).
  - This is a rename/reorganise only — no logic changes. Update import paths as needed.

---

## P3 — Polish and future-proofing

- [x] **P3-1: Upgrade Go version in `go.mod`**
  - Current: `go 1.19`. Latest stable: 1.22+.
  - Fix: update `go.mod` to `go 1.21` minimum (safe jump). Run `go mod tidy`.
  - Check if any `exportloopref` linter suppressions can be removed (fixed in Go 1.22).

- [x] **P3-2: Add Makefile with standard targets**
  - Targets: `build`, `test`, `lint`, `clean`, `release`.
  - `build` target must inject version via `-ldflags`.
  - Example:
    ```makefile
    VERSION := $(shell git describe --tags --always --dirty)
    LDFLAGS := -ldflags="-X github.com/dyegoe/awss/cmd.version=$(VERSION)"

    build:
        go build $(LDFLAGS) -o awss .

    test:
        go test ./...

    lint:
        golangci-lint run

    clean:
        rm -f awss
    ```

- [ ] **P3-3: Improve test coverage in `search/ec2` and `search/eni`**
  - Current tests focus on filters and sort validation.
  - Add tests for: nil pointer fields in AWS response, empty result sets, error propagation from AWS client.
  - Use the mockable-function pattern already established in `cmd/root.go` to inject a fake AWS client.

- [x] **P3-4: Add `sort` tag support to ENI `dataRow`**
  - ENI results currently have no sort capability.
  - Add `sort:"..."` struct tags to `eniInfo` fields (e.g. `sort:"id"`, `sort:"az"`).
  - Implement `sortResults()` for `eni.Results` mirroring the ec2 implementation.
  - Expose `--sort` flag in `cmd/eni.go`.

- [x] **P3-5: Validate output format early in `cmd/root.go`**
  - Currently, an invalid `--output` value only errors at print time, after the AWS calls have been made.
  - Fix: add validation in `persistentPreRun` using `common.ValidOutputs()`.

---

## Completion checklist

When all items above are done, verify:

- [ ] `go build ./...` — clean
- [ ] `go test ./... -race` — all pass, no race conditions
- [ ] `golangci-lint run` — zero warnings
- [ ] `go vet ./...` — clean
- [ ] Manual smoke test: `awss ec2 --help` and `awss eni --help` work correctly
- [ ] Version output (`awss --version`) shows the injected version, not `dev` or a hardcoded string
