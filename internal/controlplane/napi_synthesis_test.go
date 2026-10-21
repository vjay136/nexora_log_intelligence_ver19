package controlplane

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestNAPISynthesisCreatesEvidenceBackedDraftWithoutRawSamples(t *testing.T) {
	f := newAPIFixture(t)
	discovery := []any{
		map[string]any{"raw": `{"level":"INFO","count":12}`, "desired": map[string]any{"level": "INFO", "count": 12}},
		map[string]any{"raw": `{"level":"WARN","count":19}`, "desired": map[string]any{"level": "WARN", "count": 19}},
	}
	heldOut := []any{map[string]any{"raw": `{"level":"ERROR","count":42}`, "desired": map[string]any{"level": "ERROR", "count": 42}}}
	response, data := f.request(http.MethodPost, "/api/v1/assurance/napi/synthesize", map[string]any{
		"id": "napi-json", "name": "NAPI JSON", "version": "0.1.0",
		"discovery": discovery, "held_out": heldOut,
		"negative": []string{`{"unrelated":"value"}`},
	})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("synthesize %d: %s", response.StatusCode, data)
	}
	created := decodeMap(t, data)
	if created["state"] != "draft" || created["published"] != false || created["requires_human_approval"] != true {
		t.Fatalf("unsafe lifecycle response: %s", data)
	}
	validation, _ := created["validation"].(map[string]any)
	held, _ := validation["held_out"].(map[string]any)
	if held["exact"] != float64(1) || validation["eligible"] != true {
		t.Fatalf("held-out validation missing: %s", data)
	}
	evolution, _ := created["nupire"].(map[string]any)
	if evolution["id"] == nil || evolution["state"] != "rejected" {
		t.Fatalf("NAPI candidate was not bound to the stricter EGPE evidence gate: %s", data)
	}

	response, stored := f.request(http.MethodGet, "/api/v1/assurance/parsers/napi-json", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("get draft %d: %s", response.StatusCode, stored)
	}
	for _, raw := range []string{`{"level":"INFO","count":12}`, `{"level":"ERROR","count":42}`, `{"unrelated":"value"}`} {
		if bytes.Contains(stored, []byte(raw)) {
			t.Fatalf("raw synthesis sample leaked into parser metadata: %s", stored)
		}
		if bytes.Contains(data, []byte(raw)) {
			t.Fatalf("raw synthesis sample leaked into NUPIRE response: %s", data)
		}
	}
	if !bytes.Contains(stored, []byte(`"raw_samples_persisted":false`)) || !bytes.Contains(stored, []byte(`"held_out_sample_hashes"`)) {
		t.Fatalf("synthesis provenance missing: %s", stored)
	}

	response, publish := f.request(http.MethodPost, "/api/v1/assurance/parsers/napi-json/publish", map[string]any{})
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("draft bypassed review gate: %d %s", response.StatusCode, publish)
	}
}

func TestNAPISynthesisBuildsAnchoredRegexAndRejectsOverfit(t *testing.T) {
	f := newAPIFixture(t)
	valid := map[string]any{
		"id": "napi-regex", "name": "NAPI Regex",
		"discovery": []any{
			map[string]any{"raw": "INFO alice 100", "desired": map[string]any{"level": "INFO", "user": "alice", "count": 100}},
			map[string]any{"raw": "WARN bob 200", "desired": map[string]any{"level": "WARN", "user": "bob", "count": 200}},
		},
		"held_out": []any{map[string]any{"raw": "ERROR carol 300", "desired": map[string]any{"level": "ERROR", "user": "carol", "count": 300}}},
		"negative": []string{"unrelated record"},
	}
	response, data := f.request(http.MethodPost, "/api/v1/assurance/napi/synthesize", valid)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("regex synthesize %d: %s", response.StatusCode, data)
	}
	created := decodeMap(t, data)
	if created["strategy"] != "regex" {
		t.Fatalf("expected regex candidate: %s", data)
	}
	safety, _ := created["regex_safety"].(map[string]any)
	if safety["safe"] != true || safety["anchored"] != true || safety["predictable_runtime"] != true {
		t.Fatalf("regex safety missing: %s", data)
	}

	invalid := map[string]any{
		"id": "napi-overfit", "name": "NAPI Overfit",
		"discovery": valid["discovery"],
		"held_out":  []any{map[string]any{"raw": "ERROR|carol|300", "desired": map[string]any{"level": "ERROR", "user": "carol", "count": 300}}},
		"negative":  []string{"unrelated record"},
	}
	response, rejected := f.request(http.MethodPost, "/api/v1/assurance/napi/synthesize", invalid)
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("overfit candidate accepted: %d %s", response.StatusCode, rejected)
	}
	if !bytes.Contains(rejected, []byte(`"draft_created":false`)) {
		t.Fatalf("rejection evidence missing: %s", rejected)
	}
	response, _ = f.request(http.MethodGet, "/api/v1/assurance/parsers/napi-overfit", nil)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("rejected synthesis persisted a draft: %d", response.StatusCode)
	}
}

func TestNAPIRegexSafetyBounds(t *testing.T) {
	unsafe := analyzeNAPIRegex(map[string]any{"pattern_type": "regex", "pattern": strings.Repeat("a", napiMaxRegexBytes+1)})
	if unsafe.Safe || len(unsafe.Errors) == 0 {
		t.Fatalf("oversized regex accepted: %+v", unsafe)
	}
	safe := analyzeNAPIRegex(map[string]any{"pattern_type": "regex", "pattern": `^(?P<level>INFO|WARN|ERROR)$`})
	if !safe.Safe || !safe.Anchored || safe.Engine != "go-re2" || safe.CaptureGroups != 1 || safe.Instructions == 0 {
		encoded, _ := json.Marshal(safe)
		t.Fatalf("bounded regex rejected: %s", encoded)
	}
}

func TestNAPISynthesisRejectsDatasetLeakage(t *testing.T) {
	f := newAPIFixture(t)
	sample := map[string]any{"raw": `{"level":"INFO"}`, "desired": map[string]any{"level": "INFO"}}
	response, data := f.request(http.MethodPost, "/api/v1/assurance/napi/synthesize", map[string]any{
		"id": "napi-leak", "name": "NAPI Leakage",
		"discovery": []any{sample, map[string]any{"raw": `{"level":"WARN"}`, "desired": map[string]any{"level": "WARN"}}},
		"held_out":  []any{sample},
	})
	if response.StatusCode != http.StatusBadRequest || !bytes.Contains(data, []byte("must not contain the same normalized record")) {
		t.Fatalf("dataset leakage accepted: %d %s", response.StatusCode, data)
	}
}
