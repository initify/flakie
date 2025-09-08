# Examples

Sample packages with deterministic and intentionally flaky tests to try with flakie.

- `stringutil`: simple string helpers, tests should be stable.
- `mathutil`: simple math helpers, tests should be stable.
- `flaky`: an intentionally flaky test that fails randomly.

Run all example tests:

```
go test ./examples/...
```

Try with flakie:

```
./flakie -runs 5 -pkg ./examples/...
```
