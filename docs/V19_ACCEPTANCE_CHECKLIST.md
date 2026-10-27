# NLI Ver19 acceptance checklist

Updated: 2026-10-28

Current package identity: `19.0.0-8`. Release 8 completes the guided quarantine lifecycle with embedded candidate review, scoped parser-bundle proposals, independent rollout approval, exact client acknowledgement, replay verification, rollback, and offboarding. It retains the validation telemetry, NUPIRE/EGPE rendering, bounded parser-failure discovery, and checksum-bound candidate lifecycle from earlier releases. This note does not mark deployment-specific production acceptance.

| Gate | State | Evidence or remaining action |
|---|---|---|
| Operator and manager demo guide covers current navigation and sync behavior | PASS | `docs/V19_OPERATOR_DEMO_GUIDE.md`; `tests/v19-operator-guide.test.js` checks `NAV` IDs/titles |
| UPES bounded signatures, queue, and process-local metrics | PASS | Full Go suite passed locally for the integrated Release 8 tree; UPES runtime and metrics contracts are covered. See `docs/V19_UPES_GPRS_ARCHITECTURE_AND_ACCEPTANCE.md`. |
| GPRS checksum, withdrawal, independent approval, publish, rollback, and offboarding lifecycle | PASS | Focused controller lifecycle and browser QRW tests pass. Signed rollout/client acknowledgement remains governed by independent approval and exact-hash gates. See `docs/V19_R8_QRW_OPERATOR_TEST_GUIDE.md`. |
| Quarantine Resolution Wizard field mapping and guided projection | PASS | Exact required fixture, unresolved delimiter gate, per-field evidence, deterministic candidate generation, controller persistence, nine stages, Back/Next controls, and invalid/resource-exceeding candidate rejection are covered. See `docs/V19_QUARANTINE_RESOLUTION_WIZARD.md`. |
| Ver18 baseline fixed and preserved | PASS | Baseline commit `8a084b8`; baseline Go suite passed before changes |
| NUPIRE discovery/compiler/lifecycle | PASS | Unit, persistence, API, integration, and browser tests |
| Declarative artifact and resource limits | PASS | Compiler validation and stable error-code tests |
| Held-out evidence separation | PASS | Lifecycle and NAPI bridge tests |
| Approval/canary/promotion/drift/rollback | PASS | State-machine and controller API tests |
| Existing hot path and signed rollout preserved | PASS | Complete Go suite and architecture boundary review |
| Full Go suite and vet | PASS | `go test ./...` and `go vet ./...` passed in the pinned Go 1.25.4 Linux container |
| Browser suite | PASS | 61/61 Playwright Chromium scenarios passed on the integrated Release 8 tree, including every primary workspace, visible inner tab, and the complete guided quarantine lifecycle. |
| Hosted deployment contracts | PASS | 24/24 Node contracts; core and Vercel-mirror Vite builds passed |
| Fuzz target | PASS | 110,438 executions / 3 seconds |
| Shell and installer contracts | PASS | Syntax, client, AWS/controller edge, provenance, SELinux fixture |
| Race detector | PASS | `go test -race ./...` passed in the pinned Go 1.25.4 Linux container |
| 100 distinct upstream source reviews | PASS | 100 verified across the consolidated ledger and evidence batches; mirrors, aliases, duplicates, and incomplete inspections excluded |
| Deterministic RPM build and verification | PENDING | Runs from the frozen Release 8 commit in local release build and tag CI |
| Rocky 9/10.2 fresh install, upgrade, rollback | PENDING | Runs against the frozen Release 8 artifacts in tag CI |
| Two-node TLS/enrollment lifecycle | PENDING | Linux release integration gate |
| Reproducible artifact comparison | PENDING | Two-pass comparison runs from the frozen Release 8 commit |
| Same-host Ver18/Ver19 parser microbenchmark | PASS | Median hot-path timings level; see `evidence/V19_V18_LOCAL_PARSER_BENCHMARK.md` |
| Linux production throughput qualification | PENDING | Pinned workload required before production performance claim |

Release decision: **source release approved for tagging by the operator** after
the complete Go, browser, vet, Vite, and static packaging gates pass. Linux RPM
reproducibility, Rocky install/upgrade, race, multi-node TLS, and production
throughput qualification remain blocked or pending and must pass before the
release is described as production-qualified.
