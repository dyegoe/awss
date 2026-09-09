# Backlog — awss

Open improvements that are not tied to a user-facing feature request, plus pointers to the
feature follow-ups tracked as GitHub issues. Finished work is not listed here: the history lives
in `CHANGELOG.md` and in git.

Legend: [ ] to do · [~] in progress

---

## Feature follow-ups (tracked as issues)

- [ ] [#127](https://github.com/dyegoe/awss/issues/127) Show bucket tags in `awss s3`
  (one `GetBucketTagging` call per bucket; fetch only when `--show-tags` is set).
- [ ] [#128](https://github.com/dyegoe/awss/issues/128) `awss ec2 --cidrs` also matching
  secondary network interfaces (`network-interface.subnet-id` / `network-interface.vpc-id`).

## Reliability

- [ ] **Cancellation and timeout.** `search.Execute` uses `context.Background()`. If one region
  hangs, the whole run hangs. Wire `context.WithTimeout` (flag or config, e.g. `--timeout 60s`)
  at the top of `Execute` and pass it down; `Search(ctx)` already accepts it.
- [ ] **Pre-authenticate every profile.** The `WhoAmI` workaround only pre-authenticates the
  first profile x region. With several SSO/Okta profiles the remaining ones still authenticate
  in parallel inside the fan-out. Pre-auth each unique profile sequentially before starting
  the goroutines.

## Testing

Coverage per package as of 2026-09-09 (`go test -cover ./...`):

| Package         | Coverage | Gap                                                          |
| --------------- | -------: | ------------------------------------------------------------ |
| `common`        |    90.7% | —                                                            |
| `search/s3obj`  |    85.5% | —                                                            |
| `search/s3`     |    75.6% | `Search()` (AWS config + client construction)                |
| `search/subnet` |    68.2% | `Search()` paginator loop                                    |
| `search/vpc`    |    67.2% | `Search()` paginator loop                                    |
| `search/ec2`    |    65.6% | `Search()` DescribeInstances call                            |
| `search/ebs`    |    42.9% | `Search()`, `collectVolumeRows`, `enrichInstanceNames`       |
| `cmd`           |    40.1% | `Execute`, `persistentPreRun`, `runSearch` happy path        |
| `search/eni`    |    37.5% | `Search()` and the instance-name enrichment                  |
| `search`        |    18.3% | `Execute` fan-out                                            |

- [ ] **Inject the EC2 client** the way `search/s3` and `search/s3obj` do (the SDK's
  `Describe*APIClient` interfaces), so the `Search()` bodies of ec2, eni, ebs, vpc and subnet
  can be tested with a fake and every search package reaches the 80% target.
- [ ] **`search.Execute`**: test the fan-out with a mocked engine and a mocked `WhoAmI`.
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
