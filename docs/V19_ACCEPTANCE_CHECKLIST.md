# NLI Ver19 acceptance checklist

Updated: 2026-10-26

Current package identity: `19.0.0-6`. Release 6 adds bounded parser-failure discovery and the GPRS checksum-bound parser-candidate lifecycle, including withdrawal and rollback transitions. It retains release 5's submitted/approved/published input-path profile gate, one-client rollout targeting, a pending rollout approval count, and canonical metric-history behavior. This note does not mark full release acceptance.

| Gate | State | Evidence or remaining action |
|---|---|---|
| Operator and manager demo guide covers current navigation and sync behavior | PASS | `docs/V19_OPERATOR_DEMO_GUIDE.md`; `tests/v19-operator-guide.test.js` checks `NAV` IDs/titles |
| UPES bounded signatures, queue, and process-local metrics | PASS | Full Go suite passed locally for the integrated Release 6 revision; UPES runtime and metrics contracts are covered. See `docs/V19_UPES_GPRS_ARCHITECTURE_AND_ACCEPTANCE.md`. |
| GPRS checksum, withdrawal, independent approval, publish, and rollback lifecycle | PASS | Full Go suite passed locally for the integrated Release 6 revision, including API lifecycle tests. Signed rollout/client acknowledgement remains governed by the existing rollout acceptance gates. See `docs/V19_UPES_GPRS_ARCHITECTURE_AND_ACCEPTANCE.md`. |
| Ver18 baseline fixed and preserved | PASS | Baseline commit `8a084b8`; baseline Go suite passed before changes |
| NUPIRE discovery/compiler/lifecycle | PASS | Unit, persistence, API, integration, and browser tests |
| Declarative artifact and resource limits | PASS | Compiler validation and stable error-code tests |
| Held-out evidence separation | PASS | Lifecycle and NAPI bridge tests |
| Approval/canary/promotion/drift/rollback | PASS | State-machine and controller API tests |
| Existing hot path and signed rollout preserved | PASS | Complete Go suite and architecture boundary review |
| Full Go suite | PASS | Parent-reported full Go suite passed locally for the integrated Release 6 revision |
| Browser suite | PASS | 57/57 Playwright Chromium scenarios passed for the integrated Release 6 revision |
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

Release decision: **source release approved for tagging by the operator** after
the complete Go, browser, vet, Vite, and static packaging gates pass. Linux RPM
reproducibility, Rocky install/upgrade, race, multi-node TLS, and production
throughput qualification remain blocked or pending and must pass before the
release is described as production-qualified.
