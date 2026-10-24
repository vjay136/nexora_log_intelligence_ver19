package controlplane

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"nexora.local/nli-v19/internal/auditlog"
)

func TestCommunicationStatusUsesObservedEvidence(t *testing.T) {
	f := newAPIFixture(t)
	response, body := f.request(http.MethodGet, "/api/v1/communications/status", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("communication status %d: %s", response.StatusCode, body)
	}
	status := decodeMap(t, body)
	path, ok := status["path"].([]any)
	if !ok || len(path) != 3 {
		t.Fatalf("expected three communication hops: %s", body)
	}
	if path[0].(map[string]any)["state"] != "idle_no_registered_clients" {
		t.Fatalf("no registered clients must be idle, not live: %s", body)
	}
	if path[2].(map[string]any)["state"] != "unavailable" {
		t.Fatalf("browser must remain unavailable until heartbeat: %s", body)
	}
	response, body = f.request(http.MethodPost, "/api/v1/communications/browser-heartbeat", map[string]any{})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("browser heartbeat %d: %s", response.StatusCode, body)
	}
	response, body = f.request(http.MethodGet, "/api/v1/communications/status", nil)
	status = decodeMap(t, body)
	path = status["path"].([]any)
	if path[2].(map[string]any)["state"] != "live" {
		t.Fatalf("persisted browser heartbeat not reflected: %s", body)
	}
}

func TestNotificationPolicyAndAuditFeed(t *testing.T) {
	f := newAPIFixture(t)
	policy := map[string]any{
		"channels": map[string]any{
			"in_app":    map[string]any{"enabled": true},
			"email":     map[string]any{"enabled": true, "sender_env": "NLI_SMTP_FROM", "recipient_env": "NLI_SMTP_TO"},
			"pagerduty": map[string]any{"enabled": true, "secret_env": "NLI_PAGERDUTY_KEY", "endpoint": "https://events.pagerduty.com/v2/enqueue"},
		},
		"admin_fallback": true,
		"recipients":     []string{},
		"events":         []string{"approval", "quarantine", "sync", "system", "delivery", "security"},
	}
	response, body := f.request(http.MethodPut, "/api/v1/notifications/config", policy)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("save notification policy %d: %s", response.StatusCode, body)
	}
	if !json.Valid(body) {
		t.Fatalf("notification policy response is invalid JSON: %s", body)
	}
	response, body = f.request(http.MethodGet, "/api/v1/notifications/config", nil)
	if response.StatusCode != http.StatusOK || !json.Valid(body) {
		t.Fatalf("read notification policy %d: %s", response.StatusCode, body)
	}
	if !strings.Contains(string(body), "configuration_incomplete") {
		t.Fatalf("missing environment values must remain visibly incomplete: %s", body)
	}
	t.Setenv("NLI_SMTP_FROM", "nli@example.test")
	t.Setenv("NLI_SMTP_TO", "ops@example.test")
	t.Setenv("NLI_PAGERDUTY_KEY", "test-routing-key")
	response, body = f.request(http.MethodGet, "/api/v1/notifications/config", nil)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "configured_not_dispatched") {
		t.Fatalf("configured external channels must not claim delivery without an adapter: %s", body)
	}
	_, err := f.server.audit.Append(auditEventForTest("quarantine.reprocess", "failure"))
	if err != nil {
		t.Fatal(err)
	}
	response, body = f.request(http.MethodGet, "/api/v1/notifications", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("notification feed %d: %s", response.StatusCode, body)
	}
	feed := decodeMap(t, body)
	items, ok := feed["notifications"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("persisted quarantine audit event missing from in-app feed: %s", body)
	}
	item := items[0].(map[string]any)
	if item["category"] != "quarantine" || item["source"] != "controller_audit_chain" {
		t.Fatalf("feed item not tied to canonical audit evidence: %s", body)
	}
	response, body = f.request(http.MethodPost, "/api/v1/notifications/"+item["id"].(string), map[string]any{})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("acknowledge notification %d: %s", response.StatusCode, body)
	}
}

func auditEventForTest(action, outcome string) auditlog.Event {
	return auditlog.Event{Action: action, Resource: "test", Outcome: outcome, Actor: "admin"}
}
