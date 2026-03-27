# Code Style Guide — awss

This document defines the coding conventions for the awss project.
Claude Code enforces these rules on all new and modified code.

---

## 1. Error handling

### Always wrap errors with context

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

### Never silently discard errors

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

### Collect errors into the result, don't stop early in Search()

This is intentional in this project — one failing region should not crash the whole run.
Append to `r.Errors` and return early from the current Search() call only:

```go
if err != nil {
    r.Errors = append(r.Errors, fmt.Sprintf("describing instances: %v", err))
    return
}
```

---

## 2. Nil pointer safety

All pointer fields from AWS SDK responses must be nil-checked before dereferencing.

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

Never use `context.TODO()` in production code.

```go
// Bad
response, err := client.DescribeInstances(context.TODO(), input)

// Good — in Search() which does not yet receive a ctx
response, err := client.DescribeInstances(context.Background(), input)

// Best — when Search() is refactored to accept ctx
func (r *Results) Search(ctx context.Context) {
    response, err := client.DescribeInstances(ctx, input)
}
```

The long-term goal is for `Search()` to accept `ctx context.Context` so callers can cancel
in-flight requests. Do not block other fixes on this — use `context.Background()` as an
intermediate step.

---

## 4. Nesting depth

Maximum nesting depth is **3 levels**. Reduce nesting using:

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

Never hardcode a version string in source code.

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

The Makefile and goreleaser config must inject the version. The source file must never contain
a release version number.

---

## 6. Struct consistency: exported vs unexported

If a type is returned by an exported function, it must be exported.

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

## 7. Eliminating struct duplication with embedding

`ec2.Results` and `eni.Results` share identical fields and getter methods.
When refactoring, extract a `common.BaseResults` and embed it:

```go
// In common/results.go
type BaseResults struct {
    Profile   string   `json:"profile"`
    Region    string   `json:"region"`
    Errors    []string `json:"errors,omitempty"`
    SortField string   `json:"-"`
}

func (b *BaseResults) GetProfile() string  { return b.Profile }
func (b *BaseResults) GetRegion() string   { return b.Region }
func (b *BaseResults) GetErrors() []string { return b.Errors }
func (b *BaseResults) GetSortField() string { return b.SortField }

// In search/ec2/ec2.go
type Results struct {
    common.BaseResults
    Filters map[string][]string `json:"-"`
    Data    []dataRow           `json:"data"`
}
```

---

## 8. N+1 API call pattern

Never call AWS APIs inside a loop over results from another AWS call.

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

## 9. Naming conventions

| Thing                       | Convention             | Example                         |
| --------------------------- | ---------------------- | ------------------------------- |
| Packages                    | lowercase, single word | `ec2`, `eni`, `common`          |
| Exported types              | PascalCase             | `Results`, `BaseResults`        |
| Unexported types            | camelCase              | `dataRow`, `eniInfo`            |
| Exported functions          | PascalCase verb/noun   | `GetHeaders`, `FilterTags`      |
| Unexported functions        | camelCase              | `getFilters`, `sortResults`     |
| CLI flag labels (constants) | camelCase with prefix  | `labelProfiles`, `labelEc2Sort` |
| Test cases                  | descriptive string     | `"empty filter returns error"`  |

CLI sort/filter flag values use **kebab-case**: `private-ip`, `public-ip`, not `private_ip`.

---

## 10. Doc comments

Every exported symbol (type, function, variable, constant) must have a doc comment.
Comments start with the symbol name and are written as full sentences.

```go
// FilterTags returns a list of EC2 filter objects built from tag key=value pairs.
// It returns an empty slice if tags is empty, and an error if any tag is malformed.
func FilterTags(tags []string) ([]types.Filter, error) {
```

Unexported helpers benefit from comments too, especially if their purpose is non-obvious.
