# Ver19 subsystem names and execution order

This reference separates the client data path from controller-side improvement
workflows. A live transport badge means that current controller metadata is
arriving. It does not mean that every subsystem runs for every record.

## Names shown in the WebApp

| Short name | Full name | Responsibility |
|---|---|---|
| **NHLE** | **Nexora Hypersonic Log Engine** | Client-side source reading, framing, parsing, normalization, bounded queues, quarantine routing, and destination delivery. This is the ordinary per-record hot path. |
| **NURSF** | **Nexora Unified Real-Time Synchronization Fabric** | Authenticated canonical telemetry collection, monotonic revisioning, aggregation, incremental distribution, freshness measurement, and browser resynchronization. It transports metadata and counts; it does not reparse customer records. |
| **NAPI** | **Nexora Adaptive Parser Intelligence** | Bounded synthesis of declarative parser candidates from explicit discovery, held-out, and negative evidence. It is operator/evidence triggered and never changes a live parser directly. |
| **NAL** | **Nexora Assurance Lab** | Deterministic sample/corpus validation, retained test fixtures, test plans, run evidence, and parser review preparation. It is outside the normal ingest hot path. |
| **NUPIRE** | **Nexora Universal Parsing Intelligence & Recovery Engine** | Persists evidence-linked parser discoveries and candidate lifecycle state for recovery work. Candidate state is not production activation. |
| **EGPE** | **Evidence-Gated Parser Evolution** | Applies held-out evidence, independent review, promotion, drift, and rollback gates to parser evolution. It cannot bypass signed rollout approval or client acknowledgement. |

## Ordinary successful record

1. A configured source watcher detects appended bytes and restores its durable
   checkpoint after restart.
2. NHLE frames one single-line or multiline record and places it in a bounded
   per-source processing path.
3. The approved ParserPack registry selects and executes the eligible parser,
   validates the field contract, and normalizes the record.
4. NHLE durably writes or delivers the normalized event to configured routes.
   Acknowledgements and retries remain destination-specific.
5. The client advances the input checkpoint only after the accounted customer
   destinations are durable. Optional telemetry or optional destinations do
   not replay an already durable local output.
6. The client publishes bounded counters, rates, queue state, parser revision,
   destination acknowledgement metadata, and resource measurements. Raw
   customer log bodies are excluded from canonical telemetry.
7. NURSF authenticates the update, deduplicates and orders it, updates one
   canonical revision, and broadcasts an incremental change to WebApp
   subscribers. Command Center and subsystem pages consume this same revision.

NAPI, NAL, NUPIRE, and EGPE do not execute during this successful-record path.

## Unsupported or quarantined record recovery

1. NHLE writes an unmatched or invalid record to the local exact-raw quarantine
   queue and sends bounded metadata to the controller. Exact source evidence is
   available centrally only when encrypted raw retention was explicitly enabled.
2. An authorized operator opens **Unknown & Quarantine**, selects **View evidence
   & diagnose**, and enters the Assurance **Recovery Workspace**. View, audited
   download, exclusion/restoration, and retained-evidence deletion remain
   separate permission-checked actions.
3. NAL executes the current production parser against the selected evidence.
   The operator or NAPI can prepare a bounded declarative candidate. NAPI runs
   only when requested and retains hashes/evidence rather than silently changing
   runtime behavior.
4. NAL validates positive, held-out, negative, multiline, and malformed cases.
   A parser with failing evidence remains a draft.
5. NUPIRE/EGPE may record discovery and evaluation evidence. These lifecycle
   states support review; they do not install a parser.
6. The author submits the validated draft. A different operator reviews and
   approves or rejects it with an audit record. Only an approved candidate can
   be published.
7. Publish converts the supported declarative candidate into a strict sealed
   ParserPack, adds its immutable revision/hash to the signed fleet bundle, and
   creates the existing approval-gated rollout. Unsupported declarative forms
   fail closed instead of being interpreted differently in production.
8. After rollout approval, each selected client verifies the bundle and pack,
   applies it atomically, restarts the agent, and reports the applied bundle
   hash. Failed activation restores the prior assets and parser directory.
9. Targeted replay remains blocked until the originating client acknowledges
   the exact current bundle hash. The operator then replays selected record IDs;
   outcomes update quarantine history and canonical telemetry.
10. A successfully replayed record follows the ordinary NHLE destination path.
    Excluded or permanently purged evidence is never silently reconstructed.

## Operational ordering rule

The production dependency order is:

`source -> NHLE -> customer destination/checkpoint -> NURSF telemetry -> WebApp`

The controlled recovery loop branches only after quarantine:

`quarantine -> NAL diagnosis -> optional NAPI/NUPIRE/EGPE evidence -> independent approval -> signed fleet rollout -> client hash acknowledgement -> targeted replay -> NHLE`

The WebApp displays the authoritative state of each step. It must not infer a
completed approval, rollout, acknowledgement, or replay from elapsed time or
from the existence of a candidate.
