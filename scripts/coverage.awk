# coverage.awk passes the output of `go test -cover` through and fails when a package has no test
# files or covers less than min percent of its statements (AGENTS.md, Standards and guardrails).
# Usage: go test -cover ./... | awk -v min=80 -f scripts/coverage.awk
{ print }

# Without -cover an untested package prints "?  pkg  [no test files]"; with -cover, current Go
# prints "<tab>pkg<tab><tab>coverage: 0.0% of statements" instead, with neither "?" nor "ok".
/^\?/ && /\[no test files\]/ {
	failed = failed "\n  " $2 ": no test files"
}

/^\t/ && /coverage: [0-9.]+% of statements/ {
	failed = failed "\n  " $1 ": no test files"
}

/^ok/ && match($0, /coverage: [0-9.]+% of statements/) {
	pct = substr($0, RSTART + 10, RLENGTH - 24) + 0
	if (pct < min) {
		failed = failed sprintf("\n  %s: coverage %.1f%% is below %d%%", $2, pct, min)
	}
}

END {
	if (failed != "") {
		print "\ncoverage check failed:" failed
		exit 1
	}
}
