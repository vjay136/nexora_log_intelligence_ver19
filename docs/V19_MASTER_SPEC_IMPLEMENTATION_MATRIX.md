# Ver19 master specification implementation matrix

Updated: 2026-10-21

| Requirement | Implementation | State |
|---|---|---|
| Preserve real Ver18 runtime | Existing agent, engine, outputs, quarantine, NURSF, auth, rollout and package paths retained | Implemented |
| Universal structure discovery | `internal/nupire/discovery.go` | Implemented with bounded evidence-backed hypotheses |
| Deterministic safe compiler | `internal/nupire/compiler.go` | Implemented; declarative JSON only |
| Evidence-gated lifecycle | `internal/nupire/lifecycle.go` | Implemented with atomic persistence and audit |
| Held-out evaluation | Lifecycle evidence policy and NAPI bridge | Implemented; training and held-out IDs must be disjoint |
| Shadow/canary/promotion/rollback | Lifecycle transitions and `/api/v1/nupire` actions | Implemented; production rollout remains separately gated |
| NAPI integration | `internal/controlplane/nupire_napi_bridge.go` | Implemented without copying raw evidence |
| Controller API | `internal/controlplane/nupire_api.go` | Implemented with existing authentication/RBAC/CSRF/audit |
| Operator workspace | NUPIRE/EGPE page in `web/app.js` | Implemented with real persisted state and explicit empty/unavailable states |
| Unsupported/unstructured input | partial/abstain plus existing exact-raw quarantine/replay | Implemented safety path; universal semantics are not claimed |
| Runtime isolation | Candidate work remains off the client hot path | Implemented architecture boundary |
| 100 repository legal/research review | `V19_RESEARCH_AND_LICENSE_LEDGER.md` and batch evidence | In progress; release blocker until exact distinct count reaches 100 |
| Full RPM/Rocky release validation | CI/release workflow | Pending Linux/container execution |
| Reproducible signed release | Existing release pipeline with Ver19 identity | Pending all acceptance gates |

See [V19_NUPIRE_ARCHITECTURE.md](V19_NUPIRE_ARCHITECTURE.md) for the data and
trust boundaries and [V19_ACCEPTANCE_CHECKLIST.md](V19_ACCEPTANCE_CHECKLIST.md)
for the release decision.
