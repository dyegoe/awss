# Backlog — awss

Open improvements that are not tied to a user-facing feature request, plus pointers to the
feature follow-ups tracked as GitHub issues. Finished work is not listed here: the history lives
in `CHANGELOG.md` and in git.

Legend: [ ] to do · [~] in progress

---

## Feature follow-ups (tracked as issues)

- [ ] [Decide whether `s3obj --all` should scan region buckets](https://github.com/dyegoe/awss/issues/152).

## Testing

Coverage per package as of 2026-10-07 (`go test -cover ./...`):

| Package         | Coverage | Gap                                                          |
| --------------- | -------: | ------------------------------------------------------------ |
| `search/eni`    |    98.8% | —                                                            |
| `search/vpc`    |    98.3% | —                                                            |
| `search/ebs`    |    97.8% | —                                                            |
| `search/subnet` |    97.8% | —                                                            |
| `search/ec2`    |    97.5% | —                                                            |
| `common`        |    93.5% | —                                                            |
| `cmd`           |    92.0% | `Execute` (`os.Exit`), `BindPFlag` errors (unreachable)      |
| `search/s3obj`  |    85.5% | —                                                            |
| `search/s3`     |    80.8% | `Search()` (AWS config + client construction)                |
| `search`        |    99.1% | deadline and search ending at the same instant (not forced)  |

- [ ] [**Standardise test names and structure.**](https://github.com/dyegoe/awss/issues/173) Three
  naming styles (`TestType_Method`, `TestName`, `Test_name`), gotests `args` wrappers, shared
  mutable fixtures left in `search/eni` and `search/ebs`, and commented-out tests. Pick one
  convention, apply it, and enforce it with an automated check.

## Standards

- [ ] [**Make the standards and guardrails explicit, mandatory and enforced.**](https://github.com/dyegoe/awss/issues/174)
  One list of MUST rules in `AGENTS.md`, each with the linter, CI step or test that enforces it;
  `docs/CODESTYLE.md` and `CONTRIBUTING.md` explain and link instead of restating. Depends on #173.

## Architecture

- [ ] [**Split the `common` package.**](https://github.com/dyegoe/awss/issues/151) It holds AWS
  helpers, string utilities, tag parsing, output rendering, the `Results` interface,
  `BaseResults`, the reflection row helpers and the `Matcher`. Proposed split, rename-only:
  `awsutil/` (config, profiles, filter builders), `output/` (printers), `results/`
  (interface, `BaseResults`, row helpers), `common/` (the rest).
