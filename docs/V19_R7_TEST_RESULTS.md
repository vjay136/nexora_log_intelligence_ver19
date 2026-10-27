# Nexora Log Intelligence 19.0.0-7 test results

This report records the release gates for the exact Release 7 source tree. It
does not replace deployment acceptance against the target client's storage,
network, Loki, ClickHouse, or security controls.

## Parser and quarantine samples

The automated Go corpus exercises JSON, key/value, RFC 3164 and RFC 5424
syslog, Apache CLF, CEF, SMTP native mixed key/value, positional family
profiles, delimiter variants, multiline framing, truncated input, oversized
input, ambiguous records, and deliberately unstructured records. The fixed
migration corpus contains 678 representative records: 674 expected normalized
records and 4 expected quarantined records. The agent integration gate adds
eight reviewed-family normal records and four deliberate quarantine records,
verifies exact raw evidence retention only in the protected quarantine store,
and rejects raw payloads from telemetry and normal state.

## Quarantine recovery workflow

The browser and controller gates cover this ordered flow:

1. Retained failures are grouped by bounded structural signature.
2. An operator can view/download evidence, ignore/exclude it, return it to the
   queue, or purge retained raw evidence with the required permission.
3. GPRS accepts a field name, occurrence/index, delimiter, template, named RE2
   regular expression, desired JSON, mapping, and multi-record fixtures.
4. The production parser runs a deterministic preview. AI is offered only when
   that deterministic single-record preview fails.
5. The same screen creates or edits the declarative candidate, retests it,
   submits it, records independent approval, publishes it, displays exact
   client bundle acknowledgement, gates replay, and permits rollback.

The dedicated Quarantine Resolution Wizard adds a persistent nine-stage
progress indicator, Back/Next navigation, session-scoped mapping drafts, and a
field editor for names, labels, types, presence/null policies, extraction
method, token/key/group/path, delimiter part, transformation, timestamp format,
timezone, explicit default, bounded validation, preview, confidence, and source
evidence. The exact required `Note;systemlogs_for_data` sample remains blocked
until both semicolon mappings are explicitly confirmed.

## Release gates

| Gate | Result | Evidence |
|---|---|---|
| Focused agent, controller, and telemetry tests | PASS | Validation counters/rates, canonical aggregation, reset handling, bounded input validation, exact QRW fixture, and resource-limit rejection |
| Hosted WebApp contracts | PASS | 24/24 Node tests |
| Vite production build | PASS | Core and Vercel-mirror production bundles generated successfully |
| Full Go suite and vet | PASS | `go test ./...`; `go vet ./...` in pinned Go 1.25.4 Linux container |
| Go race detector | PASS | `go test -race ./...` in pinned Go 1.25.4 Linux container |
| Browser UI | PASS | 60/60 Playwright Chromium scenarios: every primary workspace and visible inner tab, exact QRW fixture, measured VALIDATE row, non-empty NUPIRE registry, GPRS recovery controls, request failures, and uncaught browser errors |
| Infrastructure static gate | PASS | Shell syntax; edge, SELinux rollback, AWS, Cloudflare, Vercel, installer, provenance, RPM-spec, systemd-unit, and Supabase schema/RLS/RPC contracts |
| Reproducible RPM/evidence pipeline | Required in GitHub `infrastructure-static` and release jobs | Two-pass build, hashes, manifest, SBOM, provenance, package contents |
| Rocky lifecycle | Required before publication | Rocky 9 and Rocky 10.2 install/upgrade/service/config/state checks |

The GitHub release is created only after the main-branch workflow succeeds and
the tag workflow independently completes every required job. The published
workflow links and artifact hashes are part of the release record.
