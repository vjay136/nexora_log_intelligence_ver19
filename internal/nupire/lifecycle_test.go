package nupire

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestLifecycleAndPersistentRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nupire.json")
	s, err := NewStore(path, DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	p := Provenance{Source: "masked-template:abc", Generator: "rules-v1", CreatedBy: "operator", TrainingIDs: []string{"tr-1", "tr-2"}, HeldOutIDs: []string{"ho-1", "ho-2"}}
	if _, err = s.CreateDiscovery("p-1", p); err != nil {
		t.Fatal(err)
	}
	r, err := s.SubmitCandidate("p-1", json.RawMessage(`{"kind":"key_value","version":1}`), "worker")
	if err != nil {
		t.Fatal(err)
	}
	digest := r.Digest
	if !r.DigestIsValid() {
		t.Fatal("candidate digest invalid")
	}
	m := Metrics{Samples: 20, ExactMatches: 20, FieldPrecision: 1, FieldRecall: .99, ErrorRate: 0}
	r, err = s.Evaluate("p-1", "evaluator", m, []string{"ho-1", "ho-2"})
	if err != nil {
		t.Fatal(err)
	}
	if r.State != Evaluated {
		t.Fatalf("state %s", r.State)
	}
	for _, step := range []struct {
		state State
		actor string
	}{{Shadow, "worker"}, {Approval, "reviewer"}, {Canary, "deployer"}, {Promoted, "deployer"}, {Drifted, "monitor"}, {RolledBack, "operator"}} {
		r, err = s.Transition("p-1", step.state, step.actor, "test")
		if err != nil {
			t.Fatal(err)
		}
	}
	if r.Digest != digest || !r.DigestIsValid() {
		t.Fatal("artifact digest/provenance changed")
	}
	s2, err := NewStore(path, DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s2.Get("p-1")
	if !ok || got.State != RolledBack || got.Digest != digest {
		t.Fatalf("recovered record: %+v", got)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestHeldOutIsolationAndPolicyRejection(t *testing.T) {
	s, _ := NewStore("", DefaultPolicy())
	p := Provenance{CreatedBy: "op", TrainingIDs: []string{"same"}, HeldOutIDs: []string{"held"}}
	_, _ = s.CreateDiscovery("x", p)
	_, _ = s.SubmitCandidate("x", json.RawMessage(`{"parser":"declarative"}`), "worker")
	if _, err := s.Evaluate("x", "eval", Metrics{Samples: 12, FieldPrecision: 1, FieldRecall: 1}, []string{"same"}); err == nil {
		t.Fatal("accepted overlapping held-out evidence")
	}
	r, err := s.Evaluate("x", "eval", Metrics{Samples: 12, FieldPrecision: .5, FieldRecall: .5, Ambiguous: true}, []string{"held"})
	if err != nil {
		t.Fatal(err)
	}
	if r.State != Rejected {
		t.Fatalf("state=%s", r.State)
	}
	if _, err = s.Transition("x", Shadow, "worker", ""); !errors.Is(err, ErrTransition) {
		t.Fatalf("transition error=%v", err)
	}
}

func TestIndependentApprovalAndEvidenceAreRequired(t *testing.T) {
	s, _ := NewStore("", DefaultPolicy())
	if _, err := s.CreateDiscovery("missing", Provenance{CreatedBy: "creator"}); err == nil {
		t.Fatal("accepted discovery without training and held-out evidence")
	}
	p := Provenance{CreatedBy: "creator", TrainingIDs: []string{"train"}, HeldOutIDs: []string{"held"}}
	_, _ = s.CreateDiscovery("candidate", p)
	_, _ = s.SubmitCandidate("candidate", json.RawMessage(`{"kind":"declarative"}`), "worker")
	_, _ = s.Evaluate("candidate", "evaluator", Metrics{Samples: 10, ExactMatches: 10, FieldPrecision: 1, FieldRecall: 1}, []string{"held"})
	_, _ = s.Transition("candidate", Shadow, "worker", "eligible")
	if _, err := s.Transition("candidate", Approval, "creator", "self review"); err == nil {
		t.Fatal("candidate creator approved their own candidate")
	}
	if _, err := s.Transition("candidate", Approval, "independent-reviewer", "reviewed"); err != nil {
		t.Fatal(err)
	}
}

func TestPersistentRecoveryRejectsTamperedLifecycleChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nupire.json")
	now := time.Now().UTC()
	state := diskState{Version: 1, Records: []Record{{
		ID: "tampered", State: Promoted,
		Provenance: Provenance{CreatedBy: "creator", TrainingIDs: []string{"train"}, HeldOutIDs: []string{"held"}},
		CreatedAt:  now, UpdatedAt: now,
		Events: []Event{{To: Discovery, At: now, Actor: "creator"}, {From: Discovery, To: Promoted, At: now, Actor: "creator"}},
	}}}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = NewStore(path, DefaultPolicy()); err == nil {
		t.Fatal("loaded a tampered lifecycle event chain")
	}
}

func TestConcurrentTransitionsDoNotCorruptStore(t *testing.T) {
	s, _ := NewStore("", DefaultPolicy())
	p := Provenance{CreatedBy: "op", TrainingIDs: []string{"t"}, HeldOutIDs: []string{"h"}}
	_, _ = s.CreateDiscovery("x", p)
	_, _ = s.SubmitCandidate("x", json.RawMessage(`{"kind":"declarative"}`), "worker")
	_, _ = s.Evaluate("x", "eval", Metrics{Samples: 20, ExactMatches: 20, FieldPrecision: 1, FieldRecall: 1}, []string{"h"})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.Transition("x", Shadow, "worker", ""); errs <- e }()
	}
	wg.Wait()
	close(errs)
	success := 0
	for e := range errs {
		if e == nil {
			success++
		} else if !errors.Is(e, ErrTransition) {
			t.Fatalf("unexpected error %v", e)
		}
	}
	if success != 1 {
		t.Fatalf("successful transitions=%d", success)
	}
}
