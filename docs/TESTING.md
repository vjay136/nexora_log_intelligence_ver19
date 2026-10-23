# NLI test output and package coverage

`go test ./...` prints `[no test files]` when a Go package compiles successfully but contains no package-local `_test.go` file. It is build-time test metadata, not a runtime value, synchronization status, parser count, or telemetry warning.

`[no tests to run]` has a different meaning: the package contains benchmark or specially selected test functions, but the current command did not select an ordinary `Test*` case. NLI's `internal/benchmarks` package is expected to show this during the ordinary suite because its performance workloads run through the dedicated benchmark and release gates.

NLI requires direct unit tests for reusable runtime logic and embedded release assets. The benchmark, manager CLI, migration CLI, throughput qualification, confidence, type inference, client manager, declarative parser registry, catalog, and ParserPack packages therefore have focused tests covering bounded I/O, accounting, HTTP failures, checkpoint validation, boundaries, coercion, stale-client state, concurrency-safe snapshots, parser validation, application, regression fixtures, persistence, built-in parser identity, embedded JSON validity, ordering, uniqueness, and defensive copies.

The following packages may intentionally report `[no test files]`:

- `cmd/nli-agent` and `cmd/nli-client-control`: thin executable launchers whose behavior is exercised through internal-package, integration, RPM lifecycle, and release tests.
Adding placeholder tests only to suppress the Go status would provide no regression protection. Any future reusable logic moved into these packages must include focused tests; executable wiring remains covered by the end-to-end release gates.
