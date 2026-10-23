# NLI Ver19 acceptance checklist

Updated: 2026-10-23

| Gate | State | Evidence or remaining action |
|---|---|---|
| Ver18 baseline fixed and preserved | PASS | Baseline commit `8a084b8`; baseline Go suite passed before changes |
| NUPIRE discovery/compiler/lifecycle | PASS | Unit, persistence, API, integration, and browser tests |
| Declarative artifact and resource limits | PASS | Compiler validation and stable error-code tests |
| Held-out evidence separation | PASS | Lifecycle and NAPI bridge tests |
| Approval/canary/promotion/drift/rollback | PASS | State-machine and controller API tests |
| Existing hot path and signed rollout preserved | PASS | Complete Go suite and architecture boundary review |
| Full Go suite | PASS | `go test ./...` |
| Browser suite | PASS | 42/42 Playwright Chromium |
| Hosted deployment contracts | PASS | 20/20 Node contracts; Vite build |
| Fuzz target | PASS | 110,438 executions / 3 seconds |
| Shell and installer contracts | PASS | Syntax, client, AWS/controller edge, provenance, SELinux fixture |
| Race detector | BLOCKED | Requires Linux/CGO CI environment |
| 100 distinct upstream source reviews | PASS | 100 verified across the consolidated ledger and evidence batches; mirrors, aliases, duplicates, and incomplete inspections excluded |
| Deterministic RPM build and verification | BLOCKED | Docker daemon unavailable locally; CI must run |
| Rocky 9/10.2 fresh install, upgrade, rollback | BLOCKED | Requires built RPMs and Linux hosts/containers |
| Two-node TLS/enrollment lifecycle | BLOCKED | Requires Linux integration environment |
| Reproducible artifact comparison | BLOCKED | Requires two successful release builds |
| Same-host Ver18/Ver19 parser microbenchmark | PASS | Median hot-path timings level; see `evidence/V19_V18_LOCAL_PARSER_BENCHMARK.md` |
| Linux production throughput qualification | PENDING | Pinned workload required before production performance claim |

Release decision: **BLOCKED from tagging**. Passing source tests alone does not
authorize a `v19.*` tag or GitHub release.
