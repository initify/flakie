# flakie

Flakie is a tiny tool and GitHub Action to detect flaky tests in Go projects by running tests multiple times and aggregating pass/fail per test case.

## CLI

Build:

```
go build -o flakie ./cmd/flakie
```

Run against all packages 5 times:

```
./flakie -runs 5 -pkg ./... -timeout 10m
```

JSON output:

```
./flakie -runs 5 -pkg ./... -json > flakie.json
```

Exit codes:

- 0: no flaky tests and no consistent failures
- 1: consistent failures only
- 3: flaky tests detected (regardless of consistent failures)

## GitHub Action

The workflow at `.github/workflows/flakie.yml` runs on pull requests, executes tests multiple times, and posts a sticky PR comment summarizing flaky and consistently failing tests.

Permissions: it requests `pull-requests: write` to post the comment.