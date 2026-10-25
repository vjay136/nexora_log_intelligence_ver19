# NLI Ver19 progress and acceptance state

Updated: 2026-10-25

## Release 19.0.0-4 work in progress

- The `auto` source profile no longer applies generic continuation to each
  independent line. Explicit Java/Python/Go stack rules and durable restart
  checkpoints remain enabled. The acceptance test writes 49 independent JSON
  records and checks exact-once output after restart.
- Overall Timeline History reads retained aggregate intervals. These buckets
  support exact count charts and large counter values; tests may exercise a
  simulated 10,000,000 records/second, which does not establish a production
  throughput guarantee.
- Input profiles support additive one-or-many path edits and remain subject to
  validation, independent approval, and applied-revision acknowledgement.
- Release 4 operator steps are in `docs/release/README.md` under
  “Operator acceptance for release 4.”

Ver19 is derived from released Ver18 commit `8a084b8`. The source tree is a
development candidate until every release gate below passes. This file records
measured results and does not claim universal log accuracy.

## Implemented and verified locally

- Ver18 baseline: complete `go test ./...` passed before Ver19 changes.
- Ver19 module, runtime identity, web assets, scripts, installers, RPM names,
  workflow tags, and upgrade input were migrated to `19.0.0-1`.
- NUPIRE structure discovery, bounded tokenizer/delimiter inference,
  declarative compiler, immutable digest, evidence model, lifecycle store,
  persistence recovery, approval, drift, and rollback transitions.
- NAPI held-out synthesis binds its selected declarative artifact and hashed
  evidence to the stricter NUPIRE lifecycle without retaining raw samples.
- Authenticated controller APIs and a real NUPIRE/EGPE operator workspace.
- Ver19 complete Go suite: PASS.
- Direct unit coverage for confidence routing, type inference, client-manager
  snapshots/API behavior, and declarative parser registry validation,
  application, persistence, and built-in identity: PASS. Intentional
  `[no test files]` entrypoint and embedded-asset packages are documented in
  `docs/TESTING.md` and remain covered by integration and release gates.
- NUPIRE focused tests: PASS.
- Tokenizer fuzz test: PASS, 110,438 executions in three seconds.
- Browser UI suite: PASS, 42/42.
- Vercel/Cloudflare/Vite Node contracts: PASS, 20/20.
- Vite production build: PASS.
- Shell syntax, client installer, controller edge, AWS installer, source parser
  provenance, and SELinux rollback tests: PASS.
- Same-host Ver18/Ver19 parser microbenchmarks show no material hot-path
  regression; exact samples and limits are in
  `docs/evidence/V19_V18_LOCAL_PARSER_BENCHMARK.md`.
- Runtime trace review for the 2026-10-25 incident showed no input before
  07:51:23, then 49 auto rows read and 48 emitted in one batch; the final
  row was held as a pending multiline event and emitted 21 seconds later.
  `generic_continuation` had made each line-oriented JSON row a multiline
  boundary, forcing a durable customer-output flush and checkpoint per row.
  The auto-family profile now keeps explicit Java/Python/Go stack rules while
  dropping generic continuation. NAPI, NAL, NUPIRE, and EGPE remain outside the
  ordinary successful-record ingestion path; they are used in explicit
  discovery, assurance, and recovery workflows.
- The checked-in research ledger proves 100 distinct source-level upstream
  reviews. Mirrors, aliases, duplicates, and incomplete inspections are not
  counted; per-project license and dependency caveats remain explicit.

## Pending or blocked release gates

- Docker Desktop is not running on this host. Deterministic RPM creation,
  containerized RPM/systemd checks, Rocky 9/10.2 lifecycle, two-node TLS, and
  release artifact reproduction remain pending and are also enforced by CI.
- The Windows portable Go environment cannot run `go test -race` because CGO
  is disabled. Linux CI must run the race/release gates.
- No comparative production-throughput claim is made from the local
  microbenchmark; the pinned Linux workload remains a release gate.

## Release decision

Status: **BLOCKED from tagging**, eligible for source publication and CI.

The source may be pushed so GitHub CI can execute the Linux/container gates.
A `v19.*` tag and GitHub release must wait for the RPM, lifecycle, upgrade,
rollback, race, and production-throughput evidence gates.

