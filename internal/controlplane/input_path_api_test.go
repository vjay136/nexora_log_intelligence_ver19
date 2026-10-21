package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"nexora.local/nli-v19/internal/fleet"
	metastore "nexora.local/nli-v19/internal/store"
)

func TestInputPathProfileAllows256AndRejects257Inputs(t *testing.T) {
	inputs := make([]any, 256)
	for i := range inputs {
		inputs[i] = map[string]any{"path": fmt.Sprintf("/var/log/nli/%d.log", i), "service": "auto"}
	}
	if err := validateCatalogConfig("input-paths", map[string]any{"inputs": inputs}); err != nil {
		t.Fatalf("256 input paths rejected: %v", err)
	}
	inputs = append(inputs, map[string]any{"path": "/var/log/nli/extra.log", "service": "auto"})
	if err := validateCatalogConfig("input-paths", map[string]any{"inputs": inputs}); err == nil {
		t.Fatal("257 input paths accepted")
	}
}

func TestInputPathDraftCanAddEditAndRemovePaths(t *testing.T) {
	f := newAPIFixture(t)
	response, data := f.request(http.MethodPost, "/api/v1/config/input-paths", map[string]any{
		"id": "editable-paths", "name": "Editable paths", "config": map[string]any{"inputs": []any{map[string]any{"path": "/var/log/a.log", "service": "auto"}}},
	})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create profile %d: %s", response.StatusCode, data)
	}
	response, data = f.request(http.MethodPut, "/api/v1/config/input-paths/editable-paths", map[string]any{
		"name": "Editable paths", "config": map[string]any{"inputs": []any{
			map[string]any{"path": "/var/log/a.log", "service": "auto"},
			map[string]any{"path": "/var/log/b.log", "service": "smtp-in"},
		}},
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("add path %d: %s", response.StatusCode, data)
	}
	response, data = f.request(http.MethodPut, "/api/v1/config/input-paths/editable-paths", map[string]any{
		"name": "Editable paths", "config": map[string]any{"inputs": []any{map[string]any{"path": "/var/log/b.log", "service": "smtp-in"}}},
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("remove path %d: %s", response.StatusCode, data)
	}
	profile := decodeMap(t, data)["payload"].(map[string]any)
	if len(profile["config"].(map[string]any)["inputs"].([]any)) != 1 {
		t.Fatalf("edited path set was not saved: %s", data)
	}
	response, data = f.request(http.MethodDelete, "/api/v1/config/input-paths/editable-paths", map[string]any{})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("delete draft profile %d: %s", response.StatusCode, data)
	}
}

func TestInputPathProfileRolloutTargetsSelectedOrAllAndNeedsIndependentApproval(t *testing.T) {
	f := newAPIFixture(t)
	for _, user := range []map[string]any{{"username": "path-reviewer", "email": "path-reviewer@example.test", "role": "reviewer", "password": "Path-reviewer-password-12345!"}} {
		response, data := f.request(http.MethodPost, "/api/v1/users", user)
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("create reviewer %d: %s", response.StatusCode, data)
		}
	}
	f.server.fleet.Register(&fleet.Node{ID: "path-node-1", Hostname: "node-1"})
	f.server.fleet.Register(&fleet.Node{ID: "path-node-2", Hostname: "node-2"})
	profilePayload, _ := json.Marshal(map[string]any{"name": "mail logs", "config": map[string]any{"inputs": []any{
		map[string]any{"path": "/var/log/mail/a.log", "service": "smtp-in"},
		map[string]any{"path": "/var/log/mail/b.log", "service": "smtp-out"},
	}}})
	if _, err := f.server.metadata.Put(context.Background(), metastore.Document{Collection: "input-paths", ID: "mail-profile", State: "active", Payload: profilePayload, CreatedBy: "path-author"}); err != nil {
		t.Fatal(err)
	}
	response, data := f.request(http.MethodPost, "/api/v1/config/input-paths/mail-profile/rollout", map[string]any{"node_ids": []string{"path-node-1"}})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("selected rollout %d: %s", response.StatusCode, data)
	}
	selected := decodeMap(t, data)
	selectedID := selected["id"].(string)
	selectedPayload := selected["payload"].(map[string]any)
	if selected["state"] != "pending_approval" || selectedPayload["approval_required"] != true {
		t.Fatalf("input path change bypassed independent approval: %s", data)
	}
	targets := selectedPayload["target_nodes"].([]any)
	if len(targets) != 1 || targets[0] != "path-node-1" {
		t.Fatalf("selected target nodes = %#v", targets)
	}
	rolloutPayload := selectedPayload["payload"].(map[string]any)
	patch := rolloutPayload["config_patch"].(map[string]any)
	if len(patch["inputs"].([]any)) != 2 {
		t.Fatalf("client config patch omitted profile paths: %#v", patch)
	}
	response, data = f.request(http.MethodPost, "/api/v1/rollouts/"+selectedID+"/approve", map[string]any{})
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("proposal creator approved their own rollout: %d %s", response.StatusCode, data)
	}
	f.login("path-reviewer", "Path-reviewer-password-12345!")
	f.refreshCSRF()
	response, data = f.request(http.MethodPost, "/api/v1/rollouts/"+selectedID+"/approve", map[string]any{})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("independent approval %d: %s", response.StatusCode, data)
	}
	approved := decodeMap(t, data)["payload"].(map[string]any)
	liveID := approved["live_rollout_id"].(string)
	command, ok := f.server.rollouts.Get(liveID)
	if !ok || command.Status != rolloutStatusRunning {
		t.Fatalf("approved input path rollout did not reach the client queue: %#v", command)
	}
	f.server.recordInputPathRolloutResult(liveID, "path-node-1", "completed", "configuration validated and applied")
	response, data = f.request(http.MethodGet, "/api/v1/config/input-paths/mail-profile/rollout", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("rollout history %d: %s", response.StatusCode, data)
	}
	if !bytes.Contains(data, []byte(`"input_path_sync_status":"applied"`)) {
		t.Fatalf("client result was not recorded in sync status: %s", data)
	}

	f.login("admin", "Admin-password-12345!")
	f.refreshCSRF()
	response, data = f.request(http.MethodPost, "/api/v1/config/input-paths/mail-profile/rollout", map[string]any{"node_ids": []any{}})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("all eligible rollout %d: %s", response.StatusCode, data)
	}
	all := decodeMap(t, data)["payload"].(map[string]any)
	allTargets := all["target_nodes"].([]any)
	if len(allTargets) != 2 {
		t.Fatalf("all eligible target nodes = %#v", allTargets)
	}
	pathProfile := all["input_path_profile"].(map[string]any)
	if pathProfile["target_mode"] != "all_eligible" {
		t.Fatalf("all eligible target mode missing: %#v", pathProfile)
	}
}
