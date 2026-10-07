# Code Style Guide — awss

This guide explains, with examples, the code rules of `AGENTS.md` (Standards and guardrails):
why each rule exists and how to follow it. It adds no rules of its own; when the two disagree,
`AGENTS.md` wins and this guide is the one to fix.

---

## 1. Error handling

The rule and its enforcement are in `AGENTS.md`, Standards and guardrails.

### Wrap errors with context

An error from another module says what failed, not what awss was doing. The wrap adds that:

```go
// Bad
cfg, err := common.AwsConfig(r.Profile, r.Region)
if err != nil {
    return err
}

// Good
cfg, err := common.AwsConfig(r.Profile, r.Region)
if err != nil {
    return fmt.Errorf("loading AWS config for profile %s region %s: %w", r.Profile, r.Region, err)
}
```

### Do not discard errors

```go
// Bad — parse error is thrown away, caller gets wrong behaviour silently
parsed, err := ParseTags(tags)
if err != nil {
    return filters
}

// Good — propagate the error
parsed, err := ParseTags(tags)
if err != nil {
    return nil, fmt.Errorf("parsing tag filters: %w", err)
}
```

### Collect errors into the result in Search()

One failing profile or region must not stop the others. `Search()` appends to `r.Errors` and
returns from its own call only:

```go
if err != nil {
    r.Errors = append(r.Errors, fmt.Sprintf("describing instances: %v", err))
    return
}
```

---

## 2. Nil pointer safety

The rule and its enforcement are in `AGENTS.md`, Standards and guardrails.

AWS SDK responses use pointers for most fields, and any of them can be nil (a detached ENI has
no `SubnetId`):

```go
// Bad — panics on detached ENI
SubnetID: *eni.SubnetId,

// Good
SubnetID: common.StringValue(eni.SubnetId),
```

Use `common.StringValue(ptr)` for `*string` fields.
For other pointer types, write an explicit guard:

```go
if eni.Attachment != nil && eni.Attachment.InstanceId != nil {
    row.InterfaceInfo.InstanceID = *eni.Attachment.InstanceId
}
```

---

## 3. Context

The rule and its enforcement are in `AGENTS.md`, Standards and guardrails.

`Search(ctx context.Context)` receives the context of `search.Execute`, which carries the
`--timeout` deadline. Every AWS call and every helper that makes one gets that `ctx`, so the
deadline reaches it:

```go
// Bad
response, err := client.DescribeInstances(context.TODO(), input)

// Good
func (r *Results) Search(ctx context.Context) {
    response, err := client.DescribeInstances(ctx, input)
}
```


---

## 4. Nesting depth

The rule and its enforcement are in `AGENTS.md`, Standards and guardrails. `internal/nesting`
counts `if`, `for`, `range`, `switch` and `select`; an `else if` stays at the level of its `if`,
and a function literal starts again at 0. To reduce nesting:

**Early return / guard clauses:**

```go
// Bad — 4 levels deep
for _, eni := range response.NetworkInterfaces {
    if eni.Attachment != nil {
        if eni.Attachment.InstanceId != nil {
            if name, err := lookup(id); err == nil {
                row.Name = name
            }
        }
    }
}

// Good — guard clauses flatten the logic
for _, eni := range response.NetworkInterfaces {
    if eni.Attachment == nil || eni.Attachment.InstanceId == nil {
        continue
    }
    name, err := lookup(*eni.Attachment.InstanceId)
    if err != nil {
        r.Errors = append(r.Errors, err.Error())
        continue
    }
    row.Name = name
}
```

**Extract helper functions** when a loop body grows beyond ~10 lines:

```go
func (r *Results) parseENI(eni types.NetworkInterface) (dataRow, error) { ... }
```

---

## 5. Version injection

The rule and its enforcement are in `AGENTS.md`, Standards and guardrails.

```go
// Bad
Version: "0.7.3", // TODO: Remember to update this version when releasing a new version.

// Good — in cmd/root.go
var version = "dev" // overridden at build time

var rootCmd = &cobra.Command{
    Version: version,
    ...
}
```

Build with:

```bash
go build -ldflags="-X github.com/dyegoe/awss/cmd.version=$(git describe --tags --always)" .
```

The Makefile and the release build (`.github/workflows/build-binaries.yml`, called by
`release.yml`) inject the version; the
CI smoke test builds with `-X …version=ci-test` and checks `awss --version` prints it.

---

## 6. Reuse the shared helpers

The rule and its enforcement are in `AGENTS.md`, Standards and guardrails. `common` already
provides:

| Need                                   | Helper                                              |
| -------------------------------------- | --------------------------------------------------- |
| Table headers from `header` tags       | `common.Headers(dataRow{})`                         |
| Rows for the output layer              | `common.Rows(r.Data)`                               |
| Validate a `--sort` value              | `common.SortFields(dataRow{}, f)`                   |
| List the valid sort values (help text) | `common.SortFieldNames(dataRow{})`                  |
| Sort rows by a field (numeric, slices) | `common.SortByField(r.Data, fieldName)`             |
| Client-side glob/regex matching        | `common.NewMatcher(patterns, regex)` + `.Prefix()`  |
| Validate CIDR flags                    | `common.CheckCIDRs(values)`                         |
| Build AWS filters                      | `common.FilterTags`, `FilterNames`, `FilterDefault` |

A resource package's `GetHeaders`, `GetRows`, `GetSortFields`, `SortFieldNames` and
`sortResults` should each be one line calling these.

`Results.Filters` is the same map in every profile x region goroutine of a run, so a search that
must rewrite a filter builds a copy: see `resolveCIDRFilter` in `search/ec2`.

## 7. Exported functions return exported types

The rule and its enforcement are in `AGENTS.md`, Standards and guardrails. A caller of an exported
function cannot name an unexported return type:

```go
// Bad — exported function returns unexported type
type terminalSize struct { Width, Height int }
func TerminalSize() terminalSize { ... }  // caller cannot name the return type

// Good — both exported
type TerminalSize struct { Width, Height int }
func GetTerminalSize() TerminalSize { ... }

// Also fine — both unexported (if only used internally)
type terminalSize struct { Width, Height int }
func terminalSize() terminalSize { ... }
```

---

## 8. Shared result fields: BaseResults

Every resource `Results` embeds `common.BaseResults`, which holds the profile, region, errors and
sort field and implements their getters. A new resource type embeds it too instead of
redeclaring those fields (`AGENTS.md`, Adding new resource types).

---

## 9. N+1 API call pattern

The rule and its enforcement are in `AGENTS.md`, Standards and guardrails. One call per item
multiplies the run time and the API throttling by the number of items:

```go
// Bad — one DescribeInstances call per ENI
for _, eni := range response.NetworkInterfaces {
    name, _ = searchEC2.SearchInstanceName(profile, region, *eni.Attachment.InstanceId)
}

// Good — collect all IDs, batch lookup once
instanceIDs := collectInstanceIDs(response.NetworkInterfaces)
names, err := batchLookupInstanceNames(profile, region, instanceIDs)
for _, eni := range response.NetworkInterfaces {
    row.InstanceName = names[*eni.Attachment.InstanceId]
}
```

---

## 10. Naming conventions

Go casing is checked by the compiler and `revive`; the CLI values follow the SHOULD rule of
`AGENTS.md`. Test names follow `AGENTS.md`, Testing conventions.

| Thing                       | Convention             | Example                         |
| --------------------------- | ---------------------- | ------------------------------- |
| Packages                    | lowercase, single word | `ec2`, `eni`, `common`          |
| Exported types              | PascalCase             | `Results`, `BaseResults`        |
| Unexported types            | camelCase              | `dataRow`, `eniInfo`            |
| Exported functions          | PascalCase verb/noun   | `GetHeaders`, `FilterTags`      |
| Unexported functions        | camelCase              | `getFilters`, `sortResults`     |
| CLI flag labels (constants) | camelCase with prefix  | `labelProfiles`, `labelEc2Sort` |
| Test case names             | short lower-case phrase | `"empty filter returns error"` |

CLI sort/filter flag values use **kebab-case**: `private-ip`, `public-ip`, not `private_ip`.

---

## 11. Doc comments

The rule and its enforcement are in `AGENTS.md`, Standards and guardrails. Comments start with the
symbol name and are full sentences:

```go
// FilterTags returns a list of EC2 filter objects built from tag key=value pairs.
// It returns an empty slice if tags is empty, and an error if any tag is malformed.
func FilterTags(tags []string) ([]types.Filter, error) {
```

Unexported helpers benefit from comments too, especially if their purpose is non-obvious.
