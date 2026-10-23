# NLI test output and package coverage

`go test ./...` prints `[no test files]` when a Go package compiles successfully but contains no package-local `_test.go` file. It is build-time test metadata, not a runtime value, synchronization status, parser count, or telemetry warning.

NLI requires direct unit tests for reusable runtime logic and embedded release assets. The confidence, type inference, client manager, declarative parser registry, catalog, and ParserPack packages therefore have focused tests covering boundaries, coercion, stale-client state, HTTP responses, concurrency-safe snapshots, parser validation, application, regression fixtures, persistence, built-in parser identity, embedded JSON validity, ordering, uniqueness, and defensive copies.

The following packages may intentionally report `[no test files]`:

- `cmd/nli-agent`, `cmd/nli-bench`, `cmd/nli-client-control`, `cmd/nli-manager`, `cmd/nli-migrate`, and `cmd/nli-throughput`: thin executable entrypoints whose behavior is exercised through internal-package, integration, RPM lifecycle, and release tests.
Adding placeholder tests only to suppress the Go status would provide no regression protection. Any future reusable logic moved into these packages must include focused tests; executable wiring remains covered by the end-to-end release gates.
