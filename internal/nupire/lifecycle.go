// Package nupire provides a persistent, evidence-gated parser candidate lifecycle.
// Parser artifacts are declarative data; this package never compiles or executes code.
package nupire

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type State string

const (
	Discovery  State = "discovery"
	Candidate  State = "candidate"
	Evaluated  State = "evaluated"
	Rejected   State = "rejected"
	Shadow     State = "shadow"
	Approval   State = "approval"
	Canary     State = "canary"
	Promoted   State = "promoted"
	Drifted    State = "drifted"
	RolledBack State = "rolled_back"
)

var (
	ErrNotFound   = errors.New("NUPIRE record not found")
	ErrTransition = errors.New("invalid NUPIRE state transition")
)

// Provenance identifies the evidence and process that produced the declarative artifact.
type Provenance struct {
	Source      string   `json:"source"`
	Generator   string   `json:"generator"`
	CreatedBy   string   `json:"created_by"`
	TrainingIDs []string `json:"training_ids"`
	HeldOutIDs  []string `json:"held_out_ids"`
}

// Metrics are explicit measurements, not confidence claims. Rates must be in [0,1].
type Metrics struct {
	Samples        int     `json:"samples"`
	ExactMatches   int     `json:"exact_matches"`
	FieldPrecision float64 `json:"field_precision"`
	FieldRecall    float64 `json:"field_recall"`
	ErrorRate      float64 `json:"error_rate"`
	Ambiguous      bool    `json:"ambiguous"`
}

type Policy struct {
	MinSamples        int     `json:"min_samples"`
	MinPrecision      float64 `json:"min_precision"`
	MinRecall         float64 `json:"min_recall"`
	MaxErrorRate      float64 `json:"max_error_rate"`
	MaxCandidateBytes int     `json:"max_candidate_bytes"`
	MaxEvidenceIDs    int     `json:"max_evidence_ids"`
}

func DefaultPolicy() Policy {
	return Policy{MinSamples: 10, MinPrecision: .98, MinRecall: .95, MaxErrorRate: .01, MaxCandidateBytes: 1 << 20, MaxEvidenceIDs: 10000}
}

type Event struct {
	From   State     `json:"from"`
	To     State     `json:"to"`
	At     time.Time `json:"at"`
	Actor  string    `json:"actor"`
	Reason string    `json:"reason,omitempty"`
}

type Record struct {
	ID         string          `json:"id"`
	State      State           `json:"state"`
	Artifact   json.RawMessage `json:"artifact,omitempty"`
	Digest     string          `json:"digest,omitempty"`
	Provenance Provenance      `json:"provenance"`
	Metrics    *Metrics        `json:"metrics,omitempty"`
	ApprovedBy string          `json:"approved_by,omitempty"`
	Events     []Event         `json:"events"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type diskState struct {
	Version int      `json:"version"`
	Records []Record `json:"records"`
}

type Store struct {
	mu      sync.RWMutex
	path    string
	policy  Policy
	records map[string]Record
}

func NewStore(path string, policy Policy) (*Store, error) {
	if err := validatePolicy(policy); err != nil {
		return nil, err
	}
	s := &Store{path: path, policy: policy, records: map[string]Record{}}
	if path == "" {
		return s, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var state diskState
	if err = json.Unmarshal(b, &state); err != nil {
		return nil, fmt.Errorf("decode NUPIRE state: %w", err)
	}
	if state.Version != 1 {
		return nil, fmt.Errorf("unsupported NUPIRE state version %d", state.Version)
	}
	for _, r := range state.Records {
		if err := validateRecord(r, policy); err != nil {
			return nil, fmt.Errorf("invalid persisted record %q: %w", r.ID, err)
		}
		s.records[r.ID] = clone(r)
	}
	return s, nil
}

func (s *Store) CreateDiscovery(id string, provenance Provenance) (Record, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Record{}, errors.New("record ID is required")
	}
	if len(provenance.TrainingIDs) == 0 || len(provenance.HeldOutIDs) == 0 {
		return Record{}, errors.New("training and held-out evidence identifiers are required")
	}
	if err := validateIDs(provenance.TrainingIDs, provenance.HeldOutIDs, s.policy.MaxEvidenceIDs); err != nil {
		return Record{}, err
	}
	now := time.Now().UTC()
	r := Record{ID: id, State: Discovery, Provenance: cloneProvenance(provenance), Events: []Event{{To: Discovery, At: now, Actor: provenance.CreatedBy}}, CreatedAt: now, UpdatedAt: now}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[id]; ok {
		return Record{}, fmt.Errorf("record %q already exists", id)
	}
	s.records[id] = r
	if err := s.persistLocked(); err != nil {
		delete(s.records, id)
		return Record{}, err
	}
	return clone(r), nil
}

// SubmitCandidate binds immutable declarative bytes and provenance to a discovery record.
func (s *Store) SubmitCandidate(id string, artifact json.RawMessage, actor string) (Record, error) {
	if len(artifact) == 0 || len(artifact) > s.policy.MaxCandidateBytes {
		return Record{}, errors.New("candidate artifact is empty or exceeds policy limit")
	}
	var parsed any
	if err := json.Unmarshal(artifact, &parsed); err != nil {
		return Record{}, fmt.Errorf("candidate must be declarative JSON: %w", err)
	}
	canonical, err := json.Marshal(parsed)
	if err != nil {
		return Record{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	if r.State != Discovery {
		return Record{}, ErrTransition
	}
	r.Artifact = append(json.RawMessage(nil), canonical...)
	r.Digest = artifactDigest(canonical, r.Provenance)
	r.State = Candidate
	r.UpdatedAt = time.Now().UTC()
	r.Events = append(r.Events, Event{From: Discovery, To: Candidate, At: r.UpdatedAt, Actor: actor})
	if err = s.replaceLocked(r); err != nil {
		return Record{}, err
	}
	return clone(r), nil
}

// Evaluate records held-out measurements and routes the candidate to rejection or shadow.
func (s *Store) Evaluate(id, actor string, m Metrics, heldOutIDs []string) (Record, error) {
	if err := validateMetrics(m); err != nil {
		return Record{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	if r.State != Candidate {
		return Record{}, ErrTransition
	}
	if len(heldOutIDs) == 0 || len(heldOutIDs) > s.policy.MaxEvidenceIDs {
		return Record{}, errors.New("held-out evidence identifiers are required and bounded")
	}
	if err := validateIDs(r.Provenance.TrainingIDs, heldOutIDs, s.policy.MaxEvidenceIDs); err != nil {
		return Record{}, err
	}
	if !sameIDs(r.Provenance.HeldOutIDs, heldOutIDs) {
		return Record{}, errors.New("evaluation held-out identifiers must match immutable candidate provenance")
	}
	r.Metrics = &m
	r.State = Evaluated
	r.UpdatedAt = time.Now().UTC()
	r.Events = append(r.Events, Event{From: Candidate, To: Evaluated, At: r.UpdatedAt, Actor: actor})
	if err := s.replaceLocked(r); err != nil {
		return Record{}, err
	}
	if !s.eligible(m) {
		return s.transitionLocked(id, Rejected, actor, "evidence did not meet promotion policy")
	}
	return clone(r), nil
}

func (s *Store) eligible(m Metrics) bool {
	return !m.Ambiguous && m.Samples >= s.policy.MinSamples && m.FieldPrecision >= s.policy.MinPrecision && m.FieldRecall >= s.policy.MinRecall && m.ErrorRate <= s.policy.MaxErrorRate
}

func (s *Store) Transition(id string, to State, actor, reason string) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transitionLocked(id, to, actor, reason)
}

func (s *Store) transitionLocked(id string, to State, actor, reason string) (Record, error) {
	r, ok := s.records[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	from := r.State
	allowed := map[State][]State{Evaluated: {Rejected, Shadow}, Shadow: {Approval}, Approval: {Canary}, Canary: {Promoted}, Promoted: {Drifted, RolledBack}, Drifted: {RolledBack}}
	valid := false
	for _, next := range allowed[from] {
		if next == to {
			valid = true
		}
	}
	if !valid {
		return Record{}, fmt.Errorf("%w: %s -> %s", ErrTransition, from, to)
	}
	if to == Shadow && (!r.DigestIsValid() || r.Metrics == nil || !s.eligible(*r.Metrics)) {
		return Record{}, errors.New("candidate does not satisfy evidence policy for shadow")
	}
	if to == Approval {
		if strings.TrimSpace(actor) == "" {
			return Record{}, errors.New("approval actor is required")
		}
		if actor == r.Provenance.CreatedBy {
			return Record{}, errors.New("candidate creator cannot approve their own candidate")
		}
	}
	if to == Canary && r.ApprovedBy == "" {
		return Record{}, errors.New("candidate approval is required before canary")
	}
	if to == Promoted && r.State != Canary {
		return Record{}, ErrTransition
	}
	if to == Approval {
		r.ApprovedBy = actor
	}
	r.State = to
	r.UpdatedAt = time.Now().UTC()
	r.Events = append(r.Events, Event{From: from, To: to, At: r.UpdatedAt, Actor: actor, Reason: reason})
	if err := s.replaceLocked(r); err != nil {
		return Record{}, err
	}
	return clone(r), nil
}

func (r Record) DigestIsValid() bool {
	if len(r.Artifact) == 0 || r.Digest == "" {
		return false
	}
	var v any
	if json.Unmarshal(r.Artifact, &v) != nil {
		return false
	}
	b, e := json.Marshal(v)
	if e != nil {
		return false
	}
	return artifactDigest(b, r.Provenance) == r.Digest
}
func artifactDigest(artifact []byte, p Provenance) string {
	binding := struct {
		Artifact   json.RawMessage `json:"artifact"`
		Provenance Provenance      `json:"provenance"`
	}{artifact, p}
	b, _ := json.Marshal(binding)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (s *Store) Get(id string) (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.records[id]
	return clone(r), ok
}
func (s *Store) List() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, 0, len(s.records))
	for _, r := range s.records {
		out = append(out, clone(r))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) replaceLocked(r Record) error {
	old, exists := s.records[r.ID]
	s.records[r.ID] = r
	if err := s.persistLocked(); err != nil {
		if exists {
			s.records[r.ID] = old
		} else {
			delete(s.records, r.ID)
		}
		return err
	}
	return nil
}
func (s *Store) persistLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	state := diskState{Version: 1}
	for _, r := range s.records {
		state.Records = append(state.Records, r)
	}
	sort.Slice(state.Records, func(i, j int) bool { return state.Records[i].ID < state.Records[j].ID })
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".nupire-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(name, s.path); err != nil {
		return err
	}
	return nil
}

func validatePolicy(p Policy) error {
	if p.MinSamples < 1 || p.MinSamples > 1_000_000 || p.MaxCandidateBytes < 1 || p.MaxCandidateBytes > 16<<20 || p.MaxEvidenceIDs < 1 || p.MaxEvidenceIDs > 1_000_000 || !unit(p.MinPrecision) || !unit(p.MinRecall) || !unit(p.MaxErrorRate) {
		return errors.New("invalid NUPIRE policy bounds")
	}
	return nil
}
func validateMetrics(m Metrics) error {
	if m.Samples < 0 || m.ExactMatches < 0 || m.ExactMatches > m.Samples {
		return errors.New("invalid sample counts")
	}
	for _, v := range []float64{m.FieldPrecision, m.FieldRecall, m.ErrorRate} {
		if !unit(v) {
			return errors.New("metric outside [0,1]")
		}
	}
	return nil
}
func unit(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1 }
func validateIDs(training, held []string, max int) error {
	if len(training)+len(held) > max {
		return errors.New("evidence identifier limit exceeded")
	}
	seen := map[string]bool{}
	for _, v := range training {
		if strings.TrimSpace(v) == "" || seen[v] {
			return errors.New("training evidence identifiers must be nonempty and unique")
		}
		seen[v] = true
	}
	for _, v := range held {
		if strings.TrimSpace(v) == "" || seen[v] {
			return fmt.Errorf("held-out evidence identifier %q is duplicate or overlaps training", v)
		}
		seen[v] = true
	}
	return nil
}
func uniqueSorted(ids []string) []string {
	m := map[string]bool{}
	for _, id := range ids {
		m[id] = true
	}
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
func sameIDs(a, b []string) bool {
	return strings.Join(uniqueSorted(a), "\x00") == strings.Join(uniqueSorted(b), "\x00")
}
func cloneProvenance(p Provenance) Provenance {
	p.TrainingIDs = append([]string(nil), p.TrainingIDs...)
	p.HeldOutIDs = append([]string(nil), p.HeldOutIDs...)
	return p
}
func clone(r Record) Record {
	r.Artifact = append(json.RawMessage(nil), r.Artifact...)
	r.Provenance = cloneProvenance(r.Provenance)
	r.Events = append([]Event(nil), r.Events...)
	if r.Metrics != nil {
		m := *r.Metrics
		r.Metrics = &m
	}
	return r
}
func validateRecord(r Record, p Policy) error {
	if r.ID == "" || r.State == "" {
		return errors.New("missing ID or state")
	}
	if r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) {
		return errors.New("invalid record timestamps")
	}
	if len(r.Provenance.TrainingIDs) == 0 || len(r.Provenance.HeldOutIDs) == 0 {
		return errors.New("missing evidence identifiers")
	}
	if err := validateIDs(r.Provenance.TrainingIDs, r.Provenance.HeldOutIDs, p.MaxEvidenceIDs); err != nil {
		return err
	}
	if len(r.Artifact) > p.MaxCandidateBytes {
		return errors.New("artifact exceeds policy limit")
	}
	if r.Digest != "" && !r.DigestIsValid() {
		return errors.New("artifact digest mismatch")
	}
	if _, ok := map[State]bool{Discovery: true, Candidate: true, Evaluated: true, Rejected: true, Shadow: true, Approval: true, Canary: true, Promoted: true, Drifted: true, RolledBack: true}[r.State]; !ok {
		return errors.New("unknown state")
	}
	if r.State != Discovery && (len(r.Artifact) == 0 || r.Digest == "") {
		return errors.New("candidate state is missing its bound artifact")
	}
	if r.Metrics != nil {
		if err := validateMetrics(*r.Metrics); err != nil {
			return err
		}
	}
	if r.State != Discovery && r.State != Candidate && r.Metrics == nil {
		return errors.New("evaluated state is missing metrics")
	}
	if (r.State == Canary || r.State == Promoted || r.State == Drifted || r.State == RolledBack) && strings.TrimSpace(r.ApprovedBy) == "" {
		return errors.New("post-approval state is missing its approver")
	}
	if len(r.Events) == 0 || r.Events[0].From != "" || r.Events[0].To != Discovery || r.Events[0].At.IsZero() {
		return errors.New("invalid lifecycle event origin")
	}
	previous := r.Events[0]
	for _, event := range r.Events[1:] {
		if event.From != previous.To || event.At.Before(previous.At) {
			return errors.New("invalid lifecycle event chain")
		}
		previous = event
	}
	if previous.To != r.State || r.UpdatedAt.Before(previous.At) {
		return errors.New("lifecycle event state mismatch")
	}
	return nil
}
