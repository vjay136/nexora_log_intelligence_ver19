package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssuranceScoreAndParserCandidateLifecycle(t *testing.T) {
	f := newAPIFixture(t)
	raw := `level=INFO user=alice`
	parser := map[string]any{
		"pattern_type": "regex",
		"pattern":      `level=(?P<level>\w+) user=(?P<user>\w+)`,
		"fields": []any{
			map[string]any{"name": "level", "source": "group:level", "type": "string"},
			map[string]any{"name": "user", "source": "group:user", "type": "string"},
		},
	}
	cases := []any{map[string]any{"raw": raw, "desired": map[string]any{"level": "INFO", "user": "alice"}}}
	response, data := f.request(http.MethodPost, "/api/v1/assurance/parsers", map[string]any{"name": "Key Value Sample", "parser": parser, "cases": cases})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("draft create %d: %s", response.StatusCode, data)
	}
	created := decodeMap(t, data)
	if created["state"] != "draft" {
		t.Fatalf("draft state: %s", data)
	}
	validation, _ := created["validation"].(map[string]any)
	if validation["passed"] != float64(1) || validation["failed"] != float64(0) {
		t.Fatalf("candidate validation: %s", data)
	}
	id, _ := created["id"].(string)
	response, data = f.request(http.MethodPost, "/api/v1/assurance/parsers/"+id+"/submit", map[string]any{})
	if response.StatusCode != http.StatusOK || decodeMap(t, data)["state"] != "in_review" {
		t.Fatalf("submit %d: %s", response.StatusCode, data)
	}
	response, data = f.request(http.MethodPost, "/api/v1/assurance/parsers/"+id+"/approve", map[string]any{})
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("creator must not approve their own parser, got %d: %s", response.StatusCode, data)
	}
	rejectedDraftResponse, rejectedDraftData := f.request(http.MethodPost, "/api/v1/assurance/parsers", map[string]any{"name": "Reject Candidate", "parser": parser, "cases": cases})
	if rejectedDraftResponse.StatusCode != http.StatusCreated {
		t.Fatalf("second draft create %d: %s", rejectedDraftResponse.StatusCode, rejectedDraftData)
	}
	rejectedID := decodeMap(t, rejectedDraftData)["id"].(string)
	rejectedDraftResponse, rejectedDraftData = f.request(http.MethodPost, "/api/v1/assurance/parsers/"+rejectedID+"/submit", map[string]any{})
	if rejectedDraftResponse.StatusCode != http.StatusOK {
		t.Fatalf("second draft submit %d: %s", rejectedDraftResponse.StatusCode, rejectedDraftData)
	}
	if _, err := f.server.auth.CreateUser("admin", "independent-reviewer", "reviewer@example.test", "reviewer", "Reviewer-password-12345!"); err != nil {
		t.Fatal(err)
	}
	f.login("independent-reviewer", "Reviewer-password-12345!")
	f.refreshCSRF()
	response, data = f.request(http.MethodPost, "/api/v1/assurance/parsers/"+id+"/approve", map[string]any{})
	if response.StatusCode != http.StatusOK || decodeMap(t, data)["state"] != "approved" {
		t.Fatalf("independent approval %d: %s", response.StatusCode, data)
	}
	approvedBundle, err := f.server.buildActiveParserBundle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range approvedBundle.Assets {
		if asset.AssetClass == "parser-packs" && asset.AssetID == id {
			t.Fatal("approved but unpublished Assurance parser entered runtime desired bundle")
		}
	}
	response, data = f.request(http.MethodPost, "/api/v1/assurance/parsers/"+rejectedID+"/reject", map[string]any{"reason": "Held-out cases are incomplete"})
	if response.StatusCode != http.StatusOK || decodeMap(t, data)["state"] != "rejected" {
		t.Fatalf("independent rejection %d: %s", response.StatusCode, data)
	}
	rejectedDoc, err := f.server.metadata.Get(context.Background(), "parser-packs", rejectedID, 0)
	if err != nil || !strings.Contains(string(rejectedDoc.Payload), "Held-out cases are incomplete") {
		t.Fatalf("review rejection reason not persisted: doc=%+v err=%v", rejectedDoc, err)
	}
	f.login("admin", "Admin-password-12345!")
	f.refreshCSRF()
	response, data = f.request(http.MethodPost, "/api/v1/assurance/parsers/"+id+"/publish", map[string]any{})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("publish approved runtime parser %d: %s", response.StatusCode, data)
	}
	published := decodeMap(t, data)
	if published["bundle_hash"] == "" {
		t.Fatalf("publish did not materialize signed runtime bundle: %s", data)
	}
	liveBundle, err := f.server.buildActiveParserBundle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	foundPublished := false
	for _, asset := range liveBundle.Assets {
		if asset.AssetClass == "parser-packs" && asset.AssetID == id && asset.SHA256 != "" {
			foundPublished = true
		}
		if asset.AssetClass == "parser-packs" && asset.AssetID == rejectedID {
			t.Fatal("rejected Assurance parser entered runtime desired bundle")
		}
	}
	if !foundPublished || liveBundle.BundleHash != published["bundle_hash"] {
		t.Fatalf("published parser and returned signed hash do not match bundle: found=%t bundle=%s response=%s", foundPublished, liveBundle.BundleHash, data)
	}
	response, data = f.request(http.MethodPost, "/api/v1/assurance/parsers/"+id+"/rollback", map[string]any{})
	if response.StatusCode != http.StatusOK || decodeMap(t, data)["state"] != "approved" {
		t.Fatalf("rollback active candidate %d: %s", response.StatusCode, data)
	}
	rolledBackBundle, err := f.server.buildActiveParserBundle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range rolledBackBundle.Assets {
		if asset.AssetClass == "parser-packs" && asset.AssetID == id {
			t.Fatal("rolled back parser remained in the active parser bundle")
		}
	}
	response, data = f.request(http.MethodPost, "/api/v1/assurance/run", map[string]any{"raw": `{"level":"INFO","user":"alice"}`, "desired": map[string]any{"level": "INFO", "user": "alice"}})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("assurance run %d: %s", response.StatusCode, data)
	}
	result := decodeMap(t, data)
	if result["score"] == nil || result["coverage"] == nil || result["passed"] != true {
		t.Fatalf("assurance score/coverage missing: %s", data)
	}
}

func TestAssuranceParserSubmissionCanBeCancelled(t *testing.T) {
	f := newAPIFixture(t)
	parser := map[string]any{"pattern_type": "regex", "pattern": `user=(?P<user>\w+)`, "fields": []any{map[string]any{"name": "user", "source": "group:user", "type": "string"}}}
	cases := []any{map[string]any{"raw": "user=alice", "desired": map[string]any{"user": "alice"}}}
	response, data := f.request(http.MethodPost, "/api/v1/assurance/parsers", map[string]any{"name": "Cancel candidate", "parser": parser, "cases": cases})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("draft create %d: %s", response.StatusCode, data)
	}
	id := decodeMap(t, data)["id"].(string)
	response, data = f.request(http.MethodPost, "/api/v1/assurance/parsers/"+id+"/submit", map[string]any{})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("submit %d: %s", response.StatusCode, data)
	}
	response, data = f.request(http.MethodPost, "/api/v1/assurance/parsers/"+id+"/cancel", map[string]any{})
	if response.StatusCode != http.StatusOK || decodeMap(t, data)["state"] != "draft" {
		t.Fatalf("cancel %d: %s", response.StatusCode, data)
	}
}

func TestAssuranceParserApprovalRejectsChecksumTampering(t *testing.T) {
	f := newAPIFixture(t)
	parser := map[string]any{"pattern_type": "regex", "pattern": `user=(?P<user>\w+)`, "fields": []any{map[string]any{"name": "user", "source": "group:user", "type": "string"}}}
	cases := []any{map[string]any{"raw": "user=alice", "desired": map[string]any{"user": "alice"}}}
	response, data := f.request(http.MethodPost, "/api/v1/assurance/parsers", map[string]any{"name": "Checksum candidate", "parser": parser, "cases": cases})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("draft create %d: %s", response.StatusCode, data)
	}
	id := decodeMap(t, data)["id"].(string)
	response, data = f.request(http.MethodPost, "/api/v1/assurance/parsers/"+id+"/submit", map[string]any{})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("submit %d: %s", response.StatusCode, data)
	}

	doc, err := f.server.metadata.Get(context.Background(), "parser-packs", id, 0)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(doc.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	definition := payload["parser"].(map[string]any)
	definition["pattern"] = `user=(?P<user>.+)`
	doc.Payload, _ = json.Marshal(payload)
	if _, err := f.server.metadata.Put(context.Background(), doc); err != nil {
		t.Fatal(err)
	}

	if _, err := f.server.auth.CreateUser("admin", "checksum-reviewer", "checksum-reviewer@example.test", "reviewer", "Reviewer-password-12345!"); err != nil {
		t.Fatal(err)
	}
	f.login("checksum-reviewer", "Reviewer-password-12345!")
	f.refreshCSRF()
	response, data = f.request(http.MethodPost, "/api/v1/assurance/parsers/"+id+"/approve", map[string]any{})
	if response.StatusCode != http.StatusConflict || !strings.Contains(string(data), "artifact changed") {
		t.Fatalf("tampered candidate approval must fail closed, got %d: %s", response.StatusCode, data)
	}
	bundle, err := f.server.buildActiveParserBundle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range bundle.Assets {
		if asset.AssetClass == "parser-packs" && asset.AssetID == id {
			t.Fatal("tampered candidate entered the active parser bundle")
		}
	}
}

func TestParserCreationWorkspaceDraftCanBeReadEditedAndDeleted(t *testing.T) {
	f := newAPIFixture(t)
	if err := f.server.storeQuarantineRecord(context.Background(), quarantineRecordInput{ID: "q-workspace-link", NodeID: "node-1", Status: "waiting"}, false); err != nil {
		t.Fatal(err)
	}
	parser := map[string]any{
		"pattern_type": "regex", "pattern": `status=(?P<status>\w+)`,
		"fields": []any{map[string]any{"name": "status", "source": "group:status", "type": "string"}},
	}
	cases := []any{map[string]any{"raw": "status=OK", "desired": map[string]any{"status": "OK"}}}
	response, data := f.request(http.MethodPost, "/api/v1/assurance/parsers", map[string]any{"id": "workspace-sample", "name": "Workspace sample", "version": "0.1.0", "parser": parser, "cases": cases, "source_quarantine_record_id": "q-workspace-link"})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create %d: %s", response.StatusCode, data)
	}
	response, data = f.request(http.MethodGet, "/api/v1/assurance/parsers/workspace-sample", nil)
	if response.StatusCode != http.StatusOK || !bytes.Contains(data, []byte(`"Workspace sample"`)) {
		t.Fatalf("get %d: %s", response.StatusCode, data)
	}
	if !bytes.Contains(data, []byte(`"source_quarantine_record_id":"q-workspace-link"`)) {
		t.Fatalf("draft lost its quarantine evidence association: %s", data)
	}
	response, data = f.request(http.MethodPut, "/api/v1/assurance/parsers/workspace-sample", map[string]any{"name": "Workspace revised", "version": "0.2.0", "parser": parser, "cases": cases})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("update %d: %s", response.StatusCode, data)
	}
	updated := decodeMap(t, data)
	validation, _ := updated["validation"].(map[string]any)
	if validation["score"] != float64(100) {
		t.Fatalf("update was not authoritatively validated: %s", data)
	}
	response, data = f.request(http.MethodDelete, "/api/v1/assurance/parsers/workspace-sample", map[string]any{})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("delete %d: %s", response.StatusCode, data)
	}
}

func TestAssuranceNormalizesLineEndingsAndReturnsParserCreationHelper(t *testing.T) {
	f := newAPIFixture(t)
	raw := "method=mail.message.list user=alice@example.test   \r\nfromhost=192.0.2.10\t\r\n\r\n"
	response, data := f.request(http.MethodPost, "/api/v1/assurance/run", map[string]any{
		"raw": raw, "service": "webtop-webmail", "desired": map[string]any{},
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("assurance run %d: %s", response.StatusCode, data)
	}
	result := decodeMap(t, data)
	if got, want := result["raw"], "method=mail.message.list user=alice@example.test\nfromhost=192.0.2.10"; got != want {
		t.Fatalf("normalized raw = %#v, want %#v", got, want)
	}
	helper, ok := result["parser_creation_helper"].(map[string]any)
	if !ok {
		t.Fatalf("parser_creation_helper missing: %s", data)
	}
	if helper["normalized_sample"] != result["raw"] || helper["sample_lines"] != float64(2) {
		t.Fatalf("normalized helper sample is inconsistent: %#v", helper)
	}
	if helper["review_state"] != "draft" || helper["requires_human_approval"] != true {
		t.Fatalf("helper bypassed review gate: %#v", helper)
	}
	fields, _ := helper["fields"].([]any)
	if len(fields) == 0 {
		t.Fatalf("helper did not expose detected fields: %#v", helper)
	}
}

func TestDedicatedParserHelperReturnsCompleteCopyableCategories(t *testing.T) {
	f := newAPIFixture(t)
	raw := "level=INFO user=alice\r\n"
	response, data := f.request(http.MethodPost, "/api/v1/assurance/parser-helper", map[string]any{"raw": raw, "service": "auto", "desired": map[string]any{"level": "INFO"}})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("helper response %d: %s", response.StatusCode, data)
	}
	result := decodeMap(t, data)
	if result["raw"] != "level=INFO user=alice" || result["requires_human_approval"] != true {
		t.Fatalf("helper did not preserve normalized raw input and review gate: %s", data)
	}
	parsed, ok := result["parsed_json"].(map[string]any)
	if !ok || parsed["level"] != "INFO" {
		t.Fatalf("parsed JSON missing: %s", data)
	}
	helper := result["parser_creation_helper"].(map[string]any)
	if helper["normalized_raw_input"] != result["raw"] || helper["parsed_json"] == nil || helper["family_decision"] == nil || helper["validation_accounting"] == nil {
		t.Fatalf("helper omitted its normalized input, family decision, parsed result, or accounting: %s", data)
	}
	categories, ok := result["categorized_results"].([]any)
	if !ok || len(categories) != 6 {
		t.Fatalf("helper returned %d categorized artifacts, want 6: %s", len(categories), data)
	}
	for _, rawCategory := range categories {
		category := rawCategory.(map[string]any)
		if category["copy_json"] == nil || category["artifact"] == nil {
			t.Fatalf("artifact is not copyable: %#v", category)
		}
	}
}

func TestPromoteRetainedAssuranceCaseValidatesAndStoresOnlyProvenance(t *testing.T) {
	f := newAPIFixture(t)
	response, data := f.request(http.MethodPost, "/api/v1/assurance/cases", map[string]any{
		"name": "retained-source", "retain": true,
		"case": map[string]any{"raw": "status=OK", "desired": map[string]any{"status": "OK"}},
	})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("retain fixture %d: %s", response.StatusCode, data)
	}
	id := decodeMap(t, data)["id"].(string)
	raw := "status=OK"
	parser := map[string]any{"pattern_type": "regex", "pattern": `status=(?P<status>\w+)`, "fields": []any{map[string]any{"name": "status", "source": "group:status", "type": "string"}}}
	cases := []any{map[string]any{"raw": raw, "desired": map[string]any{"status": "OK"}}}
	response, data = f.request(http.MethodPost, "/api/v1/assurance/cases/"+id+"/promote", map[string]any{"name": "Retained status parser", "parser": parser, "cases": cases})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("promote fixture %d: %s", response.StatusCode, data)
	}
	created := decodeMap(t, data)
	draftResponse, draftData := f.request(http.MethodGet, "/api/v1/assurance/parsers/retained-status-parser", nil)
	if draftResponse.StatusCode != http.StatusOK {
		t.Fatalf("read draft %d: %s", draftResponse.StatusCode, draftData)
	}
	draft := decodeMap(t, draftData)
	payload, _ := json.Marshal(draft["payload"])
	if bytes.Contains(payload, []byte(raw)) || !bytes.Contains(payload, []byte(`"validation_passed":true`)) || !bytes.Contains(payload, []byte(`"activated":false`)) {
		t.Fatalf("draft must contain passing validation and inactive state without raw samples: %s", payload)
	}
	if created["review_state"] != "draft" {
		t.Fatalf("promotion did not remain a draft: %s", data)
	}
}

func TestPromoteRetainedAssuranceCaseRejectsUnsafeAndKeepsFailingCandidateDraft(t *testing.T) {
	f := newAPIFixture(t)
	response, data := f.request(http.MethodPost, "/api/v1/assurance/cases", map[string]any{"name": "retained-source", "retain": true, "case": map[string]any{"raw": "status=OK"}})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("retain fixture %d: %s", response.StatusCode, data)
	}
	id := decodeMap(t, data)["id"].(string)
	cases := []any{map[string]any{"raw": "status=OK", "desired": map[string]any{"status": "OK"}}}
	for name, parser := range map[string]map[string]any{
		"unsafe":  {"command": "echo unsafe"},
		"invalid": {"pattern_type": "regex", "pattern": "status=(?P<status>", "fields": []any{map[string]any{"name": "status", "source": "group:status"}}},
	} {
		response, data = f.request(http.MethodPost, "/api/v1/assurance/cases/"+id+"/promote", map[string]any{"name": "Rejected " + name, "parser": parser, "cases": cases})
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s parser status=%d body=%s", name, response.StatusCode, data)
		}
	}
	response, data = f.request(http.MethodPost, "/api/v1/assurance/cases/"+id+"/promote", map[string]any{"name": "Failing candidate", "parser": map[string]any{"pattern_type": "regex", "pattern": `status=(?P<status>\w+)`, "fields": []any{map[string]any{"name": "status", "source": "group:status", "type": "string"}}}, "cases": []any{map[string]any{"raw": "status=BAD", "desired": map[string]any{"status": "OK"}}}})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("failing candidate should remain reviewable as draft: %d: %s", response.StatusCode, data)
	}
	created := decodeMap(t, data)
	if created["review_state"] != "draft" {
		t.Fatalf("failed candidate bypassed draft review: %s", data)
	}
}

func TestAssurancePlanCRUDKeepsImmutableRevisions(t *testing.T) {
	f := newAPIFixture(t)
	plan := map[string]any{"objective": "Qualify parser changes", "environment": "staging", "scenarios": []any{"parser-regression"}, "pass_criteria": []any{"all fixtures pass"}}
	response, data := f.request(http.MethodPost, "/api/v1/assurance/plans", map[string]any{"id": "parser-qualification", "name": "Parser qualification", "version": "1.2.0", "plan": plan})
	if response.StatusCode != http.StatusCreated || decodeMap(t, data)["revision"] != float64(1) {
		t.Fatalf("create revision 1 status=%d body=%s", response.StatusCode, data)
	}
	plan["objective"] = "Qualify revised parser changes"
	response, data = f.request(http.MethodPut, "/api/v1/assurance/plans/parser-qualification", map[string]any{"name": "Parser qualification", "version": "1.3.0", "plan": plan})
	if response.StatusCode != http.StatusCreated || decodeMap(t, data)["revision"] != float64(2) {
		t.Fatalf("create revision 2 status=%d body=%s", response.StatusCode, data)
	}
	response, data = f.request(http.MethodGet, "/api/v1/assurance/plans/parser-qualification?revision=1", nil)
	if response.StatusCode != http.StatusOK || !bytes.Contains(data, []byte("Qualify parser changes")) || bytes.Contains(data, []byte("Qualify revised")) {
		t.Fatalf("historical revision was not immutable: %d %s", response.StatusCode, data)
	}
	response, data = f.request(http.MethodGet, "/api/v1/assurance/plans", nil)
	if response.StatusCode != http.StatusOK || decodeMap(t, data)["count"] != float64(1) || !bytes.Contains(data, []byte(`"revision":2`)) {
		t.Fatalf("list latest plans status=%d body=%s", response.StatusCode, data)
	}
	response, data = f.request(http.MethodDelete, "/api/v1/assurance/plans/parser-qualification", map[string]any{})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("delete plan status=%d body=%s", response.StatusCode, data)
	}
	response, data = f.request(http.MethodGet, "/api/v1/assurance/plans/parser-qualification", nil)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted plan is still current: %d %s", response.StatusCode, data)
	}
}

func TestAssurancePlanSchemaAndRBAC(t *testing.T) {
	f := newAPIFixture(t)
	validPlan := map[string]any{"objective": "Test", "environment": "staging", "scenarios": []any{"parse"}, "pass_criteria": []any{"pass"}}
	for _, plan := range []map[string]any{
		{"objective": "Test", "environment": "staging", "scenarios": []any{}, "pass_criteria": []any{"pass"}},
		{"objective": "Test", "environment": "staging", "scenarios": []any{"parse"}, "pass_criteria": []any{"pass"}, "command": "unsafe"},
		{"objective": "Test", "environment": "staging", "scenarios": []any{"parse"}, "pass_criteria": []any{"pass"}, "required_features": strings.Repeat("x", 129)},
	} {
		response, data := f.request(http.MethodPost, "/api/v1/assurance/plans", map[string]any{"name": "Invalid", "plan": plan})
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("invalid plan accepted: status=%d body=%s", response.StatusCode, data)
		}
	}
	if _, err := f.server.auth.CreateUser("admin", "plan-viewer", "plan-viewer@example.test", "viewer", "Viewer-password-123!"); err != nil {
		t.Fatal(err)
	}
	viewer := &apiFixture{t: f.t, server: f.server, http: f.http, client: f.client, public: f.public, private: f.private, state: f.state}
	viewer.login("plan-viewer", "Viewer-password-123!")
	viewer.refreshCSRF()
	response, data := viewer.request(http.MethodPost, "/api/v1/assurance/plans", map[string]any{"name": "Viewer plan", "plan": validPlan})
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer created plan: status=%d body=%s", response.StatusCode, data)
	}
	response, data = viewer.request(http.MethodGet, "/api/v1/assurance/plans", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("viewer could not read plans: status=%d body=%s", response.StatusCode, data)
	}
}

func TestAssuranceMixedLineEndingDetectionAndExactLineCounts(t *testing.T) {
	f := newAPIFixture(t)
	raw := "first  \t\r\n\r\nsecond\t\nthird   \r\n\t\n"
	response, data := f.request(http.MethodPost, "/api/v1/assurance/run", map[string]any{"raw": raw})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("assurance run %d: %s", response.StatusCode, data)
	}
	result := decodeMap(t, data)
	if got, want := result["raw"], "first\n\nsecond\nthird"; got != want {
		t.Fatalf("normalized raw = %#v, want %#v", got, want)
	}
	endings, ok := result["line_endings"].(map[string]any)
	if !ok || endings["crlf"] != float64(3) || endings["lf"] != float64(2) || endings["cr"] != float64(0) {
		t.Fatalf("submitted line ending counts inaccurate: %#v", result["line_endings"])
	}
	helper := result["parser_creation_helper"].(map[string]any)
	if helper["sample_lines"] != float64(4) {
		t.Fatalf("normalized sample line count = %#v, want 4", helper["sample_lines"])
	}
}

func TestAIProviderProtocolAdapters(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		path           string
		expectedHeader string
		response       map[string]any
	}{
		{name: "OpenAI compatible", provider: "openai", path: "/chat/completions", expectedHeader: "Authorization", response: map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{"status":"ok"}`}}}}},
		{name: "Anthropic", provider: "anthropic", path: "/messages", expectedHeader: "X-API-Key", response: map[string]any{"content": []any{map[string]any{"type": "text", "text": `{"status":"ok"}`}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != test.path {
					t.Errorf("path = %q, want %q", r.URL.Path, test.path)
				}
				if r.Header.Get(test.expectedHeader) == "" {
					t.Errorf("missing %s header", test.expectedHeader)
				}
				if test.provider == "anthropic" && r.Header.Get("Anthropic-Version") != "2023-06-01" {
					t.Errorf("Anthropic-Version = %q", r.Header.Get("Anthropic-Version"))
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(test.response)
			}))
			defer server.Close()
			cfg := aiProviderConfig{Provider: test.provider, BaseURL: server.URL, Model: "test-model", APIKey: "test-api-key"}
			if err := validateProvider(cfg); err != nil {
				t.Fatalf("validateProvider: %v", err)
			}
			content, err := (&Server{}).callAI(context.Background(), cfg, "return JSON")
			if err != nil {
				t.Fatalf("callAI: %v", err)
			}
			if content != `{"status":"ok"}` {
				t.Fatalf("content = %q", content)
			}
		})
	}
}

func TestAIProviderDefaultsAndAllowlist(t *testing.T) {
	openAI := normalizeProvider(aiProviderConfig{Provider: "openai", Model: "gpt-test", APIKey: "test-api-key"})
	if openAI.BaseURL != "https://api.openai.com/v1" || validateProvider(openAI) != nil {
		t.Fatalf("OpenAI default was not valid: %#v", openAI)
	}
	anthropic := normalizeProvider(aiProviderConfig{Provider: "anthropic", Model: "claude-test", APIKey: "test-api-key"})
	if anthropic.BaseURL != "https://api.anthropic.com/v1" || validateProvider(anthropic) != nil {
		t.Fatalf("Anthropic default was not valid: %#v", anthropic)
	}
	if err := validateProvider(aiProviderConfig{Provider: "arbitrary", BaseURL: "https://example.net", Model: "x", APIKey: "test-api-key"}); err == nil {
		t.Fatal("unsupported provider was accepted")
	}
	if err := validateProvider(normalizeProvider(aiProviderConfig{Provider: "custom", Model: "x", APIKey: "test-api-key"})); err == nil {
		t.Fatal("custom provider without endpoint was accepted")
	}
	if err := validateProvider(aiProviderConfig{Provider: "custom", BaseURL: "http://provider.example.com/v1", Model: "x", APIKey: "test-api-key"}); err == nil {
		t.Fatal("non-loopback plaintext provider endpoint was accepted")
	}
}

func TestAIProviderStatusNeverReturnsCredential(t *testing.T) {
	f := newAPIFixture(t)
	secret := "provider-secret-value-1234"
	if err := f.server.storeAIProvider(context.Background(), aiProviderConfig{Provider: "openai", BaseURL: "https://api.openai.com/v1", Model: "test-model", APIKey: secret}, "admin"); err != nil {
		t.Fatal(err)
	}
	response, data := f.request(http.MethodGet, "/api/v1/admin/ai-provider", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("provider status %d: %s", response.StatusCode, data)
	}
	if bytes.Contains(data, []byte(secret)) || bytes.Contains(data, []byte(`"api_key"`)) || bytes.Contains(data, []byte(`"ciphertext"`)) {
		t.Fatalf("provider status disclosed secret material: %s", data)
	}
	if !bytes.Contains(data, []byte(`"key_last4":"1234"`)) {
		t.Fatalf("provider status did not return the bounded credential hint: %s", data)
	}
}

func TestAIFailureProposalIsDiagnosticReviewOnlyAndDoesNotRetainRaw(t *testing.T) {
	f := newAPIFixture(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{"name":"SMTP status extractor","description":"Add a bounded status capture and validate it against the failed sample.","parser":{"pattern_type":"regex","pattern":"status=(?P<status>\\w+)","fields":[{"name":"status","source":"group:status","type":"string"}]},"regex":{"pattern":"status=(?P<status>\\w+)","fields":[{"name":"status","source":"group:status","type":"string"}]},"template":{"pattern_type":"regex","pattern":"status=(?P<status>\\w+)"},"kv":{"delimiter":"="},"field_mapping":{"status":"group:status"},"projection":{"fields":["status"]}}`}}}})
	}))
	defer provider.Close()
	if err := f.server.storeAIProvider(context.Background(), aiProviderConfig{Provider: "openai", BaseURL: provider.URL, Model: "test-model", APIKey: "test-api-key"}, "admin"); err != nil {
		t.Fatal(err)
	}
	raw := "user=private@example.com status=FAILED"
	response, data := f.request(http.MethodPost, "/api/v1/assurance/ai-proposal", map[string]any{"raw": raw, "service": "smtp", "desired": map[string]any{"status": "DELIVERED"}})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("AI proposal %d: %s", response.StatusCode, data)
	}
	created := decodeMap(t, data)
	if created["review_state"] != "draft" || created["requires_human_approval"] != true || created["published"] != false {
		t.Fatalf("AI proposal bypassed review: %s", data)
	}
	if suggestions, ok := created["suggestions"].(map[string]any); !ok || len(suggestions) != 6 || created["categories"] == nil {
		t.Fatalf("AI response omitted structured category suggestions: %s", data)
	}
	response, stored := f.request(http.MethodGet, "/api/v1/assurance/ai-proposals", nil)
	if response.StatusCode != http.StatusOK || bytes.Contains(stored, []byte(raw)) || bytes.Contains(stored, []byte("private@example.com")) {
		t.Fatalf("AI proposal store retained raw customer data: %d %s", response.StatusCode, stored)
	}
	if !bytes.Contains(stored, []byte(`"diagnosis"`)) || !bytes.Contains(stored, []byte(`"suggested_method"`)) {
		t.Fatalf("AI proposal diagnosis is incomplete: %s", stored)
	}
	id, _ := created["id"].(string)
	response, data = f.request(http.MethodPost, "/api/v1/assurance/ai-proposals/"+id+"/approve", map[string]any{})
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("proposal creator approved their own AI proposal: %d %s", response.StatusCode, data)
	}
}
