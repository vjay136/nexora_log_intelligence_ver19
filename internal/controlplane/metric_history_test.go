package controlplane

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nexora.local/nli-v19/internal/fleet"
	"nexora.local/nli-v19/telemetry"
)

func TestMetricHistoryMonotonicResetZeroAndHighCounts(t *testing.T) {
	h, err := newMetricHistory(filepath.Join(t.TempDir(), "history.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 12, 0, 15, 0, time.UTC)
	sample := telemetry.StreamFileTelemetry{NodeID: "n", Source: "s", ProcessGeneration: "generation-1", ReadLines: 300_000_000, Normalized: 299_500_000, Quarantined: 500_000}
	if err := h.observe([]telemetry.StreamFileTelemetry{sample}, now); err != nil {
		t.Fatal(err)
	}
	sample.ReadLines = 600_000_000
	sample.Normalized = 599_000_000
	sample.Quarantined = 1_000_000
	if err := h.observe([]telemetry.StreamFileTelemetry{sample}, now.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	got := h.window(now.Truncate(time.Minute), now.Truncate(time.Minute).Add(time.Minute), time.Minute)
	if len(got) != 1 || got[0].Read != 600_000_000 || got[0].Parsed != 599_000_000 {
		t.Fatalf("monotonic deltas: %+v", got)
	}
	if float64(got[0].Read)/60 != 10_000_000 {
		t.Fatalf("10M/s representation changed: %v", float64(got[0].Read)/60)
	}
	// A decrease in the same generation is a counter reset; the new cumulative
	// value starts a fresh delta interval instead of underflowing.
	sample.ReadLines = 7
	sample.Normalized = 6
	sample.Quarantined = 1
	if err := h.observe([]telemetry.StreamFileTelemetry{sample}, now.Add(35*time.Second)); err != nil {
		t.Fatal(err)
	}
	got = h.window(now.Truncate(time.Minute), now.Truncate(time.Minute).Add(time.Minute), time.Minute)
	if got[0].Read != 600_000_007 {
		t.Fatalf("reset delta=%d", got[0].Read)
	}
	// A process restart also starts from the received cumulative count.
	sample.ProcessGeneration = "generation-2"
	sample.ReadLines = 11
	if err := h.observe([]telemetry.StreamFileTelemetry{sample}, now.Add(40*time.Second)); err != nil {
		t.Fatal(err)
	}
	got = h.window(now.Truncate(time.Minute), now.Truncate(time.Minute).Add(time.Minute), time.Minute)
	if got[0].Read != 600_000_018 {
		t.Fatalf("restart delta=%d", got[0].Read)
	}
}

func TestMetricsTimeSeriesExposesCanonicalIntervalCounts(t *testing.T) {
	f := newAPIFixture(t)
	now := time.Now().UTC()
	if err := f.server.metricHistory.observe([]telemetry.StreamFileTelemetry{{NodeID: "n", Source: "s", ProcessGeneration: "generation-1", ReadLines: 600, Normalized: 580, Quarantined: 20, Errors: 3}}, now); err != nil {
		t.Fatal(err)
	}
	response, data := f.request(http.MethodGet, "/api/v1/metrics/timeseries?range=1h&step=1m", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("timeseries status=%d: %s", response.StatusCode, data)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	history := result["canonical_history"].(map[string]any)
	if history["bucket_width_seconds"] != float64(60) || history["retention_seconds"] != float64(metricHistoryRetention.Seconds()) {
		t.Fatalf("history contract: %#v", history)
	}
	points := result["points"].([]any)
	var found bool
	for _, raw := range points {
		p := raw.(map[string]any)
		if p["received"] == float64(600) {
			found = true
			if p["parsed"] != float64(580) || p["quarantine"] != float64(20) || p["errors"] != float64(3) || p["logs_received"] != float64(600) {
				t.Fatalf("canonical interval point: %#v", p)
			}
		}
	}
	if !found {
		t.Fatalf("canonical counts missing from points: %s", data)
	}
}

func TestStreamTelemetryAcceptanceSurvivesHistoryPersistenceFailure(t *testing.T) {
	f := newAPIFixture(t)
	f.server.fleet.Register(&fleet.Node{ID: "history-failure-node", Hostname: "history-failure-node"})
	const token = "history-failure-node-token"
	if err := f.server.fleet.SetToken("history-failure-node", token); err != nil {
		t.Fatal(err)
	}
	blockedPath := filepath.Join(t.TempDir(), "history-is-a-directory")
	if err := os.Mkdir(blockedPath, 0700); err != nil {
		t.Fatal(err)
	}
	f.server.metricHistory.path = blockedPath
	now := time.Now().UTC()
	body, err := json.Marshal(map[string]any{
		"schema_version": telemetry.SyncSchemaVersion, "node_id": "history-failure-node", "process_generation": "generation-123",
		"sequence": 1, "collected_at": now, "published_at": now, "collection_interval_ms": 1000, "publication_interval_ms": 1000, "full_snapshot": true,
		"stream_files": []any{map[string]any{"source": "/var/log/input.log", "phase": "reading", "read_lines": 42}},
	})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, f.http.URL+"/api/v1/node/stream-telemetry", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("accepted batch failed due to retained-history error: status=%d", response.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "accepted" {
		t.Fatalf("batch was not accepted: %#v", result)
	}
	files := f.server.streamTelemetry.Snapshot()
	if len(files) != 1 || files[0].ReadLines != 42 {
		t.Fatalf("canonical live state lost accepted batch: %+v", files)
	}
}

func TestMetricHistoryZeroTrafficPersistenceAndRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	h, err := newMetricHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	if err := h.observe([]telemetry.StreamFileTelemetry{{NodeID: "idle", Source: "s", ProcessGeneration: "generation-1"}}, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := h.window(now.Add(-time.Minute), now, time.Minute); len(got) != 1 || got[0].Read != 0 {
		t.Fatalf("zero traffic fabricated: %+v", got)
	}
	sample := telemetry.StreamFileTelemetry{NodeID: "n", Source: "s", ProcessGeneration: "generation-1", ReadLines: 100, Normalized: 90}
	if err := h.observe([]telemetry.StreamFileTelemetry{sample}, now); err != nil {
		t.Fatal(err)
	}
	buckets := h.window(now, now.Add(time.Minute), time.Minute)
	if len(buckets) != 1 || buckets[0].Read != 100 {
		t.Fatalf("initial counter baseline: %+v", buckets)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var encoded metricHistoryFile
	if err := json.Unmarshal(data, &encoded); err != nil || len(encoded.Buckets) != 2 || len(encoded.Sources) != 2 {
		t.Fatalf("persisted buckets: %s (%v)", data, err)
	}
	reloaded, err := newMetricHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.window(now, now.Add(time.Minute), time.Minute); len(got) != 1 || got[0].Read != 100 {
		t.Fatalf("reload: %+v", got)
	}
	sample.ReadLines = 102
	if err := reloaded.observe([]telemetry.StreamFileTelemetry{sample}, now.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	if got := reloaded.window(now, now.Add(time.Minute), time.Minute); got[0].Read != 102 {
		t.Fatalf("restart duplicated cumulative count: %+v", got)
	}
	reloaded.mu.Lock()
	reloaded.buckets[now.Add(-metricHistoryRetention-time.Minute).Unix()] = &historyBucket{At: now.Add(-metricHistoryRetention - time.Minute)}
	reloaded.mu.Unlock()
	if err := reloaded.observe(nil, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	reloaded.mu.Lock()
	_, exists := reloaded.buckets[now.Add(-metricHistoryRetention-time.Minute).Unix()]
	reloaded.mu.Unlock()
	if exists {
		t.Fatal("expired bucket retained")
	}
}
