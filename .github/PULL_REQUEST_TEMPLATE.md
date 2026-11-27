<!--
Title must follow Conventional Commits, e.g. `feat(search/vpc): add --owner-ids filter`.
PRs are squash-merged, so the title becomes the commit message and drives the release notes.
-->

## What

<!-- What changes and why. Link the issue: Closes #123 -->

## Where this sits on the map

<!-- Which parts of docs/ARCHITECTURE.md this change touches (command, run, fan-out, search,
output, a package). When the structure changes, a small before/after, and update the file. -->

## Docs and comments re-read

<!-- Every doc section and code comment the change made you re-read, each "still true" or
"updated": README.md, AGENTS.md, docs/ARCHITECTURE.md, the comments of the changed code. -->

## Verification

- [ ] `make build`, `make test` and `make lint` pass
- [ ] Tests added or updated for the change
- [ ] `README.md` updated if a flag, command, sort field or config key changed
- [ ] `docs/ARCHITECTURE.md` updated if a piece of the map moved, was added or was removed
- [ ] Commit(s) follow the Conventional Commits format (`docs/CODESTYLE.md`, `CONTRIBUTING.md`)
