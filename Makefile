SHELL := bash
.SHELLFLAGS := -eo pipefail -c

VERSION := $(shell git describe --tags --always --dirty)

# COVERAGE_MIN is the coverage every package must reach (AGENTS.md, Standards and guardrails).
COVERAGE_MIN := 80
LDFLAGS := -ldflags="-X github.com/dyegoe/awss/cmd.version=$(VERSION)"

.PHONY: build test lint clean commit

build:
	go build $(LDFLAGS) -o awss .

# test is the one test command: CI, the pre-commit hook and AGENTS.md all run it. Random order
# and every test twice (a failure prints the seed: rerun with -shuffle=<seed>), the race
# detector, and the coverage and test-file check.
test:
	go test -race -count=2 -shuffle=on -timeout 2m -short -cover ./... | awk -v min=$(COVERAGE_MIN) -f scripts/coverage.awk

lint:
	golangci-lint run

clean:
	rm -f awss

commit:
	cz commit
