// Package upes tracks bounded, privacy-preserving parsing failure signatures
// for asynchronous discovery. It does not retain raw records or run recovery
// work on the ingestion path.
package upes

import (
	"strings"
	"sync"
	"time"
)

const (
	DefaultSignatureLimit = 4096
	DefaultQueueLimit     = 256
	maxTextBytes          = 128
)

type Failure struct {
	Signature string    `json:"signature"`
	Family    string    `json:"family"`
	Class     string    `json:"class"`
	Reason    string    `json:"reason,omitempty"`
	Count     uint64    `json:"count"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

type Stats struct {
	UniqueSignatures   uint64 `json:"unique_signatures"`
	ObservedFailures   uint64 `json:"observed_failures"`
	QueuedDiscoveries  uint64 `json:"queued_discoveries"`
	DroppedDiscoveries uint64 `json:"dropped_discoveries"`
	PendingDiscoveries int    `json:"pending_discoveries"`
}

// Runtime aggregates signature counts and queues only the first occurrence of
// each signature. Both retained signatures and pending work have hard bounds.
type Runtime struct {
	mu         sync.Mutex
	limit      int
	queueLimit int
	entries    map[string]Failure
	order      []string
	queue      []Failure
	stats      Stats
}

func New(signatureLimit, queueLimit int) *Runtime {
	if signatureLimit < 1 {
		signatureLimit = DefaultSignatureLimit
	}
	if queueLimit < 1 {
		queueLimit = DefaultQueueLimit
	}
	return &Runtime{limit: signatureLimit, queueLimit: queueLimit, entries: make(map[string]Failure, signatureLimit)}
}

// Observe records metadata only. A raw message, sample, or source path is never
// accepted or retained by this API.
func (r *Runtime) Observe(signature, family, class, reason string) {
	if r == nil {
		return
	}
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return
	}
	now := time.Now().UTC()
	family, class, reason = bounded(family), bounded(class), boundedDiagnostic(reason)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.ObservedFailures++
	if existing, ok := r.entries[signature]; ok {
		existing.Count++
		existing.LastSeen = now
		if existing.Reason == "" {
			existing.Reason = reason
		}
		r.entries[signature] = existing
		return
	}
	entry := Failure{Signature: signature, Family: family, Class: class, Reason: reason, Count: 1, FirstSeen: now, LastSeen: now}
	if len(r.entries) == r.limit {
		oldest := r.order[0]
		r.order = r.order[1:]
		delete(r.entries, oldest)
	}
	r.entries[signature] = entry
	r.order = append(r.order, signature)
	r.stats.UniqueSignatures++
	if len(r.queue) == r.queueLimit {
		r.stats.DroppedDiscoveries++
		return
	}
	r.queue = append(r.queue, entry)
	r.stats.QueuedDiscoveries++
}

// Next returns one deduplicated discovery summary. Returned data contains no
// source record content and is safe to hand to an isolated worker.
func (r *Runtime) Next() (Failure, bool) {
	if r == nil {
		return Failure{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.queue) == 0 {
		return Failure{}, false
	}
	item := r.queue[0]
	r.queue[0] = Failure{}
	r.queue = r.queue[1:]
	return item, true
}

func (r *Runtime) Stats() Stats {
	if r == nil {
		return Stats{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.stats
	out.PendingDiscoveries = len(r.queue)
	return out
}

func bounded(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > maxTextBytes {
		return value[:maxTextBytes]
	}
	return value
}

func boundedDiagnostic(value string) string {
	value = bounded(value)
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("._:-/", char) {
			continue
		}
		return "diagnostic_detail_redacted"
	}
	return value
}
