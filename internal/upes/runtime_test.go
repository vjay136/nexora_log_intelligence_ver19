package upes

import (
	"sync"
	"testing"
)

func TestObserveAggregatesAndQueuesOnlyFirstSignature(t *testing.T) {
	r := New(8, 4)
	r.Observe("fp-a", "auth", "no_parser_match", "")
	r.Observe("fp-a", "auth", "no_parser_match", "new reason")
	got, ok := r.Next()
	if !ok || got.Signature != "fp-a" || got.Count != 1 {
		t.Fatalf("queued discovery should be first-seen snapshot: %+v ok=%v", got, ok)
	}
	stats := r.Stats()
	if stats.UniqueSignatures != 1 || stats.ObservedFailures != 2 || stats.QueuedDiscoveries != 1 || stats.PendingDiscoveries != 0 {
		t.Fatalf("unexpected aggregate stats: %+v", stats)
	}
	if _, ok := r.Next(); ok {
		t.Fatal("repeated failure scheduled duplicate discovery")
	}
}

func TestQueueAndSignatureRetentionAreBounded(t *testing.T) {
	r := New(2, 1)
	r.Observe("one", "svc", "no_parser_match", "")
	r.Observe("two", "svc", "no_parser_match", "")
	r.Observe("three", "svc", "no_parser_match", "")
	stats := r.Stats()
	if stats.UniqueSignatures != 3 || stats.DroppedDiscoveries != 2 || stats.PendingDiscoveries != 1 {
		t.Fatalf("limits not enforced: %+v", stats)
	}
	if _, ok := r.Next(); !ok {
		t.Fatal("first queued signature missing")
	}
	r.Observe("three", "svc", "no_parser_match", "")
	if got := r.Stats(); got.UniqueSignatures != 3 || got.ObservedFailures != 4 {
		t.Fatalf("bounded signature eviction/count failed: %+v", got)
	}
}

func TestObserveConcurrentCallsRemainConsistent(t *testing.T) {
	r := New(32, 32)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				r.Observe("same", "svc", "no_parser_match", "")
			}
		}()
	}
	wg.Wait()
	stats := r.Stats()
	if stats.UniqueSignatures != 1 || stats.ObservedFailures != 1600 || stats.QueuedDiscoveries != 1 {
		t.Fatalf("concurrent aggregate mismatch: %+v", stats)
	}
}

func TestObserveRedactsFreeFormDiagnosticText(t *testing.T) {
	r := New(8, 4)
	r.Observe("fp", "smtp", "parser_error", "could not parse customer=secret@example.test")
	got, ok := r.Next()
	if !ok || got.Reason != "diagnostic_detail_redacted" {
		t.Fatalf("free-form reason was not redacted: %+v", got)
	}
}
