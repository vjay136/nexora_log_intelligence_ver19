package telemetry

import "time"

const SyncSchemaVersion = "nli.sync.v1"

const (
	FreshnessLive             = "live"
	FreshnessDelayed          = "delayed"
	FreshnessStale            = "stale"
	FreshnessDisconnected     = "disconnected"
	FreshnessUnavailable      = "unavailable"
	FreshnessNotInstrumented  = "not_instrumented"
	FreshnessCollectionFailed = "collection_failed"
)

// SyncBatch is the compact, authenticated client-to-controller publication.
// Sequence is scoped to (node_id, process_generation), so restarts and retries
// are distinguishable without interpreting cumulative counters as new data.
type SyncBatch struct {
	SchemaVersion         string                `json:"schema_version"`
	NodeID                string                `json:"node_id"`
	ProcessGeneration     string                `json:"process_generation"`
	Sequence              uint64                `json:"sequence"`
	CollectedAt           time.Time             `json:"collected_at"`
	PublishedAt           time.Time             `json:"published_at"`
	CollectionIntervalMS  int64                 `json:"collection_interval_ms"`
	PublicationIntervalMS int64                 `json:"publication_interval_ms"`
	FullSnapshot          bool                  `json:"full_snapshot"`
	StreamFiles           []StreamFileTelemetry `json:"stream_files"`
}

// SyncIssue is bounded operational evidence. It never carries raw log data or
// arbitrary labels, and its correlation ID is generated from safe identifiers.
type SyncIssue struct {
	Code          string    `json:"code"`
	Stage         string    `json:"stage"`
	Severity      string    `json:"severity"`
	Component     string    `json:"component"`
	NodeID        string    `json:"node_id,omitempty"`
	Source        string    `json:"source,omitempty"`
	Reason        string    `json:"reason"`
	Evidence      string    `json:"evidence,omitempty"`
	CorrelationID string    `json:"correlation_id"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
	Occurrences   uint64    `json:"occurrences"`
}

type SyncDiagnostics struct {
	TargetIntervalMS       int64       `json:"target_interval_ms"`
	LiveThresholdMS        int64       `json:"live_threshold_ms"`
	DelayedThresholdMS     int64       `json:"delayed_threshold_ms"`
	StaleThresholdMS       int64       `json:"stale_threshold_ms"`
	Subscribers            int         `json:"subscribers"`
	TrackedNodes           int         `json:"tracked_nodes"`
	TrackedSources         int         `json:"tracked_sources"`
	LiveSources            int         `json:"live_sources"`
	DelayedSources         int         `json:"delayed_sources"`
	StaleSources           int         `json:"stale_sources"`
	DisconnectedSources    int         `json:"disconnected_sources"`
	AcceptedBatches        uint64      `json:"accepted_batches"`
	DuplicateBatches       uint64      `json:"duplicate_batches"`
	OutOfOrderBatches      uint64      `json:"out_of_order_batches"`
	SequenceGaps           uint64      `json:"sequence_gaps"`
	Reconnects             uint64      `json:"reconnects"`
	CoalescedUpdates       uint64      `json:"coalesced_updates"`
	RejectedBatches        uint64      `json:"rejected_batches"`
	MissedFreshnessTargets uint64      `json:"missed_freshness_targets"`
	ClockSkewSamples       uint64      `json:"clock_skew_samples"`
	ObservedDelayP50MS     float64     `json:"observed_delay_p50_ms,omitempty"`
	ObservedDelayP95MS     float64     `json:"observed_delay_p95_ms,omitempty"`
	ObservedDelayP99MS     float64     `json:"observed_delay_p99_ms,omitempty"`
	ControllerP50MS        float64     `json:"controller_processing_p50_ms,omitempty"`
	ControllerP95MS        float64     `json:"controller_processing_p95_ms,omitempty"`
	ControllerP99MS        float64     `json:"controller_processing_p99_ms,omitempty"`
	DelayMeasurement       string      `json:"delay_measurement"`
	Issues                 []SyncIssue `json:"issues,omitempty"`
}

type SyncAggregate struct {
	SourceRecordsReadTotal      uint64  `json:"nli_source_records_read_total"`
	SourceBytesReadTotal        uint64  `json:"nli_source_bytes_read_total"`
	ParserSucceededTotal        uint64  `json:"nli_parser_records_succeeded_total"`
	ParserFailedTotal           uint64  `json:"nli_parser_records_failed_total"`
	NormalizedRecordsTotal      uint64  `json:"nli_normalized_records_total"`
	QuarantinedRecordsTotal     uint64  `json:"nli_quarantined_records_total"`
	ValidationAttemptsTotal     uint64  `json:"nli_validation_attempts_total"`
	ValidationPassedTotal       uint64  `json:"nli_validation_passed_total"`
	ValidationFailedTotal       uint64  `json:"nli_validation_failed_total"`
	QueueDepth                  int     `json:"nli_queue_depth"`
	ReadLinesPerSecond          float64 `json:"read_lines_per_second"`
	NormalizedPerSecond         float64 `json:"normalized_per_second"`
	QuarantinedPerSecond        float64 `json:"quarantined_per_second"`
	ValidationAttemptsPerSecond float64 `json:"validation_attempts_per_second"`
	ValidationPassedPerSecond   float64 `json:"validation_passed_per_second"`
	ValidationFailedPerSecond   float64 `json:"validation_failed_per_second"`
	BytesPerSecond              float64 `json:"bytes_per_second"`
	BackpressureSourceCount     int     `json:"backpressure_source_count"`
	TelemetryLastSuccessUnix    int64   `json:"nli_telemetry_last_success_timestamp"`
	PropagationDelaySecondsP95  float64 `json:"nli_telemetry_propagation_delay_seconds_p95,omitempty"`
}

type SyncSnapshot struct {
	SchemaVersion    string                `json:"schema_version"`
	Revision         uint64                `json:"revision"`
	GeneratedAt      time.Time             `json:"generated_at"`
	TargetIntervalMS int64                 `json:"target_interval_ms"`
	Aggregate        SyncAggregate         `json:"aggregate"`
	Sources          []StreamFileTelemetry `json:"sources"`
	Diagnostics      SyncDiagnostics       `json:"diagnostics"`
}

type SyncIngestResult struct {
	Status       string `json:"status"`
	Revision     uint64 `json:"revision"`
	Sequence     uint64 `json:"sequence"`
	SequenceGaps uint64 `json:"sequence_gaps,omitempty"`
	Accepted     int    `json:"accepted_sources"`
}
