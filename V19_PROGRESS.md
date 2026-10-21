# NLI Ver19 progress and acceptance state

Updated: 2026-10-21

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

## Pending or blocked release gates

- The prompt requires 100 distinct source-level repository reviews. The
  checked-in ledger proves exactly 77 distinct reviews; 23 remain. The count
  must not be described as 100 until all evidence rows exist.
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
A `v19.*` tag and GitHub release must wait for the research, RPM, lifecycle,
upgrade, rollback, and benchmark evidence gates.

