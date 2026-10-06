# Backlog — awss

Open improvements that are not tied to a user-facing feature request, plus pointers to the
feature follow-ups tracked as GitHub issues. Finished work is not listed here: the history lives
in `CHANGELOG.md` and in git.

Legend: [ ] to do · [~] in progress

---

## Feature follow-ups (tracked as issues)

None open.

## Reliability

- [ ] **Cancellation and timeout.** `search.Execute` uses `context.Background()`. If one region
  hangs, the whole run hangs. Wire `context.WithTimeout` (flag or config, e.g. `--timeout 60s`)
  at the top of `Execute` and pass it down; `Search(ctx)` already accepts it.
- [ ] **Pre-authenticate every profile.** The `WhoAmI` workaround only pre-authenticates the
  first profile x region. With several SSO/Okta profiles the remaining ones still authenticate
  in parallel inside the fan-out. Pre-auth each unique profile sequentially before starting
  the goroutines.

## Testing

Coverage per package as of 2026-10-06 (`go test -cover ./...`):

| Package         | Coverage | Gap                                                          |
| --------------- | -------: | ------------------------------------------------------------ |
| `common`        |    91.3% | —                                                            |
| `search/s3obj`  |    85.5% | —                                                            |
| `search/s3`     |    80.8% | `Search()` (AWS config + client construction)                |
| `search/subnet` |    73.8% | `Search()` paginator loop                                    |
| `search/ec2`    |    72.2% | `Search()` DescribeInstances call                            |
| `search/vpc`    |    62.3% | `Search()` paginator loop                                    |
| `search/eni`    |    48.5% | `Search()` and the instance-name enrichment                  |
| `search/ebs`    |    42.9% | `Search()`, `collectVolumeRows`, `enrichInstanceNames`       |
| `cmd`           |    39.5% | `Execute`, `persistentPreRun`, `runSearch` happy path        |
| `search`        |   100.0% | —                                                            |

- [ ] **Inject the EC2 client** the way `search/s3` and `search/s3obj` do (the SDK's
  `Describe*APIClient` interfaces), so the `Search()` bodies of ec2, eni, ebs, vpc and subnet
  can be tested with a fake and every search package reaches the 80% target. Start with
  `search/eni` (48.5%): its `Search()` body, including the instance-name lookup, is untested.
- [ ] **`cmd`**: drive `rootCmd` with `ExecuteC()` in tests and capture stdout.
- [ ] **Race check in CI**: `go test -race ./...` passes locally; add it to
  `.github/workflows/common.yml`.

## Architecture

- [ ] **Split the `common` package.** It holds AWS helpers, string utilities, tag parsing,
  output rendering, the `Results` interface, `BaseResults`, the reflection row helpers and the
  `Matcher`. Proposed split, rename-only: `awsutil/` (config, STS, profiles, filter builders),
  `output/` (printers), `results/` (interface, `BaseResults`, row helpers), `common/` (the rest).
- [ ] **`--all` for `s3obj`?** Today `--buckets` is required. Decide whether scanning every
  bucket of a region without naming them is wanted, and what `--max-keys` should default to then.

## CI / release

- [ ] Cache Go modules and build cache in `common.yml` (`actions/setup-go` `cache: true`).
- [ ] Run the unit tests on the two latest Go minors as a matrix.
