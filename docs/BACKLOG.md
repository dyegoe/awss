# Backlog — awss

Open improvements that are not tied to a user-facing feature request, plus pointers to the
feature follow-ups tracked as GitHub issues. Finished work is not listed here: the history lives
in `CHANGELOG.md` and in git.

Legend: [ ] to do · [~] in progress

---

## Feature follow-ups (tracked as issues)

- [ ] [Decide whether `s3obj --all` should scan region buckets](https://github.com/dyegoe/awss/issues/152).

## Reliability

- [ ] [**Cancellation and timeout.**](https://github.com/dyegoe/awss/issues/142) `search.Execute` uses `context.Background()`. If one region
  hangs, the whole run hangs. Wire `context.WithTimeout` (flag or config, e.g. `--timeout 60s`)
  at the top of `Execute` and pass it down; `Search(ctx)` already accepts it.
- [ ] [**Pre-authenticate every profile.**](https://github.com/dyegoe/awss/issues/143) The `WhoAmI` workaround only pre-authenticates the
  first profile x region. With several SSO/Okta profiles the remaining ones still authenticate
  in parallel inside the fan-out. Pre-auth each unique profile sequentially before starting
  the goroutines.

## Testing

Coverage per package as of 2026-10-06 (`go test -cover ./...`):

| Package         | Coverage | Gap                                                          |
| --------------- | -------: | ------------------------------------------------------------ |
| `search/eni`    |    98.8% | —                                                            |
| `search/vpc`    |    98.3% | —                                                            |
| `search/ebs`    |    97.8% | —                                                            |
| `search/subnet` |    97.8% | —                                                            |
| `search/ec2`    |    97.5% | —                                                            |
| `common`        |    91.3% | —                                                            |
| `search/s3obj`  |    85.5% | —                                                            |
| `search/s3`     |    80.8% | `Search()` (AWS config + client construction)                |
| `cmd`           |    39.8% | `Execute`, `persistentPreRun`, `runSearch` happy path        |
| `search`        |   100.0% | —                                                            |

- [ ] [**`cmd`: test root command execution.**](https://github.com/dyegoe/awss/issues/149)
  Drive `rootCmd` with `ExecuteC()` in tests and capture stdout.

## Architecture

- [ ] [**Split the `common` package.**](https://github.com/dyegoe/awss/issues/151) It holds AWS
  helpers, string utilities, tag parsing, output rendering, the `Results` interface,
  `BaseResults`, the reflection row helpers and the `Matcher`. Proposed split, rename-only:
  `awsutil/` (config, STS, profiles, filter builders), `output/` (printers), `results/`
  (interface, `BaseResults`, row helpers), `common/` (the rest).
