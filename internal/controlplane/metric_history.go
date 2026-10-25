package controlplane

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nexora.local/nli-v19/telemetry"
)

const metricHistoryRetention = 30 * 24 * time.Hour

type historySource struct {
	Generation string `json:"generation"`
	Read       uint64 `json:"read"`
	Parsed     uint64 `json:"parsed"`
	Quarantine uint64 `json:"quarantine"`
	Errors     uint64 `json:"errors"`
}

type historyBucket struct {
	At         time.Time `json:"timestamp"`
	Read       uint64    `json:"read"`
	Parsed     uint64    `json:"parsed"`
	Quarantine uint64    `json:"quarantine"`
	Errors     uint64    `json:"errors"`
}

type metricHistoryFile struct {
	Version int                      `json:"version"`
	Buckets []historyBucket          `json:"buckets"`
	Sources map[string]historySource `json:"sources"`
}

// metricHistory retains only fixed-width aggregates. Counter deltas are derived
// from the canonical latest source counters, never from event records.
type metricHistory struct {
	mu           sync.Mutex
	path         string
	sources      map[string]historySource
	buckets      map[int64]*historyBucket
	lastPersist  time.Time
	lastObserved time.Time
}

func newMetricHistory(path string) (*metricHistory, error) {
	h := &metricHistory{path: path, sources: map[string]historySource{}, buckets: map[int64]*historyBucket{}}
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		var saved metricHistoryFile
		if err := json.Unmarshal(b, &saved); err != nil {
			return nil, err
		}
		h.sources = saved.Sources
		if h.sources == nil {
			h.sources = map[string]historySource{}
		}
		for i := range saved.Buckets {
			item := saved.Buckets[i]
			if time.Since(item.At) <= metricHistoryRetention {
				h.buckets[item.At.Unix()] = &item
			}
		}
	}
	return h, nil
}

func (h *metricHistory) observe(sources []telemetry.StreamFileTelemetry, now time.Time) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	now = now.UTC()
	h.lastObserved = now
	var rd, parsed, quarantine, errs uint64
	updated := make(map[string]historySource, len(sources))
	for _, sample := range sources {
		key := sample.NodeID + "\x00" + sample.Source
		old, ok := h.sources[key]
		if !ok || old.Generation != sample.ProcessGeneration {
			var err error
			if rd, err = addCount(rd, sample.ReadLines); err != nil {
				return err
			}
			if parsed, err = addCount(parsed, sample.Normalized); err != nil {
				return err
			}
			if quarantine, err = addCount(quarantine, sample.Quarantined); err != nil {
				return err
			}
			if errs, err = addCount(errs, sample.Errors); err != nil {
				return err
			}
		} else {
			var err error
			if rd, err = addCount(rd, counterDelta(old.Read, sample.ReadLines)); err != nil {
				return err
			}
			if parsed, err = addCount(parsed, counterDelta(old.Parsed, sample.Normalized)); err != nil {
				return err
			}
			if quarantine, err = addCount(quarantine, counterDelta(old.Quarantine, sample.Quarantined)); err != nil {
				return err
			}
			if errs, err = addCount(errs, counterDelta(old.Errors, sample.Errors)); err != nil {
				return err
			}
		}
		updated[key] = historySource{sample.ProcessGeneration, sample.ReadLines, sample.Normalized, sample.Quarantined, sample.Errors}
	}
	key := now.Truncate(time.Minute).Unix()
	b := h.buckets[key]
	if b == nil {
		b = &historyBucket{At: time.Unix(key, 0).UTC()}
		h.buckets[key] = b
	}
	readTotal, err := addCount(b.Read, rd)
	if err != nil {
		return err
	}
	parsedTotal, err := addCount(b.Parsed, parsed)
	if err != nil {
		return err
	}
	quarantineTotal, err := addCount(b.Quarantine, quarantine)
	if err != nil {
		return err
	}
	errorTotal, err := addCount(b.Errors, errs)
	if err != nil {
		return err
	}
	b.Read, b.Parsed, b.Quarantine, b.Errors = readTotal, parsedTotal, quarantineTotal, errorTotal
	for key, source := range updated {
		h.sources[key] = source
	}
	cutoff := now.Add(-metricHistoryRetention).Unix()
	for k := range h.buckets {
		if k < cutoff {
			delete(h.buckets, k)
		}
	}
	// Persist at most once per bucket, keeping the retained state bounded and
	// avoiding a disk write on every high-rate telemetry publication.
	if h.lastPersist.IsZero() || now.Sub(h.lastPersist) >= time.Minute {
		if err := h.persistLocked(); err != nil {
			return err
		}
		h.lastPersist = now
	}
	return nil
}

func addCount(a, b uint64) (uint64, error) {
	if ^uint64(0)-a < b {
		return 0, errors.New("canonical metric counter aggregation exceeds uint64")
	}
	return a + b, nil
}

func counterDelta(previous, current uint64) uint64 {
	if current < previous {
		return current
	}
	return current - previous
}

func (h *metricHistory) persistLocked() error {
	items := make([]historyBucket, 0, len(h.buckets))
	for _, b := range h.buckets {
		items = append(items, *b)
	}
	data, err := json.Marshal(metricHistoryFile{Version: 1, Buckets: items, Sources: h.sources})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0750); err != nil {
		return err
	}
	tmp := h.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, h.path)
}

func (h *metricHistory) window(from, to time.Time, step time.Duration) []historyBucket {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]historyBucket, 0, int(to.Sub(from)/step))
	for at := from; at.Before(to); at = at.Add(step) {
		var bucket historyBucket
		bucket.At = at
		start := at.Unix()
		end := at.Add(step).Unix()
		for minute := start; minute < end; minute += 60 {
			if b := h.buckets[minute]; b != nil {
				bucket.Read += b.Read
				bucket.Parsed += b.Parsed
				bucket.Quarantine += b.Quarantine
				bucket.Errors += b.Errors
			}
		}
		out = append(out, bucket)
	}
	return out
}
