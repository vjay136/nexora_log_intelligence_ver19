package nupire

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	MaxPatternBytes = 4096
	MaxFields       = 128
	MaxTokens       = 1024
	MaxNesting      = 32
	MaxCaptures     = 128
	MaxRecordBytes  = 1 << 20
)

type ParserKind string

const (
	ParserRegex     ParserKind = "regex"
	ParserDelimited ParserKind = "delimited"
	ParserJSON      ParserKind = "json"
)

// ParserSpec is data-only. Pattern uses Go's RE2-based regexp package; code fields are unsupported.
type ParserSpec struct {
	Version   int         `json:"version"`
	ID        string      `json:"id"`
	Kind      ParserKind  `json:"kind"`
	Pattern   string      `json:"pattern,omitempty"`
	Delimiter string      `json:"delimiter,omitempty"`
	Fields    []FieldSpec `json:"fields"`
}

type FieldSpec struct {
	Name     string `json:"name"`
	Capture  int    `json:"capture,omitempty"`
	Index    int    `json:"index,omitempty"`
	Source   string `json:"source,omitempty"`
	Required bool   `json:"required,omitempty"`
}

type CompileError struct{ Code string }

func (e *CompileError) Error() string { return e.Code }

type CompiledParser struct {
	spec     ParserSpec
	re       *regexp.Regexp
	digest   string
	maxBytes int
}

func Compile(spec ParserSpec) (*CompiledParser, error) {
	if spec.Version != 1 || !safeID(spec.ID) || len(spec.Fields) == 0 || len(spec.Fields) > MaxFields {
		return nil, &CompileError{"schema_invalid"}
	}
	seen := map[string]bool{}
	for _, f := range spec.Fields {
		if !safeID(f.Name) || seen[f.Name] {
			return nil, &CompileError{"field_schema_invalid"}
		}
		seen[f.Name] = true
	}
	cp := &CompiledParser{spec: cloneSpec(spec), maxBytes: MaxRecordBytes}
	switch spec.Kind {
	case ParserRegex:
		if len(spec.Pattern) == 0 || len(spec.Pattern) > MaxPatternBytes || spec.Delimiter != "" {
			return nil, &CompileError{"pattern_limit"}
		}
		if nestingDepth(spec.Pattern) > MaxNesting {
			return nil, &CompileError{"nesting_limit"}
		}
		if !validRegexCaptures(spec.Pattern, MaxCaptures) {
			return nil, &CompileError{"capture_limit"}
		}
		re, err := regexp.Compile(spec.Pattern)
		if err != nil {
			return nil, &CompileError{"pattern_syntax"}
		}
		if re.NumSubexp() > MaxCaptures {
			return nil, &CompileError{"capture_limit"}
		}
		cp.re = re
		for _, f := range spec.Fields {
			if f.Capture < 1 || f.Capture > re.NumSubexp() || f.Index != 0 || f.Source != "" {
				return nil, &CompileError{"capture_schema_invalid"}
			}
		}
	case ParserDelimited:
		if len([]rune(spec.Delimiter)) != 1 || spec.Pattern != "" {
			return nil, &CompileError{"delimiter_invalid"}
		}
		for _, f := range spec.Fields {
			if f.Index < 0 || f.Index >= MaxTokens || f.Capture != 0 || f.Source != "" {
				return nil, &CompileError{"field_index_invalid"}
			}
		}
	case ParserJSON:
		if spec.Pattern != "" || spec.Delimiter != "" {
			return nil, &CompileError{"json_schema_invalid"}
		}
		for _, f := range spec.Fields {
			if f.Source == "" || f.Capture != 0 || f.Index != 0 {
				return nil, &CompileError{"json_source_invalid"}
			}
		}
	default:
		return nil, &CompileError{"kind_unsupported"}
	}
	canonical, err := json.Marshal(spec)
	if err != nil {
		return nil, &CompileError{"schema_invalid"}
	}
	sum := sha256.Sum256(canonical)
	cp.digest = hex.EncodeToString(sum[:])
	return cp, nil
}

func (p *CompiledParser) Digest() string { return p.digest }

type Outcome string

const (
	OutcomeParsed  Outcome = "parsed"
	OutcomePartial Outcome = "partial"
	OutcomeAbstain Outcome = "abstain"
)

type ParseResult struct {
	Outcome   Outcome        `json:"outcome"`
	Fields    map[string]any `json:"fields"`
	Missing   []string       `json:"missing,omitempty"`
	ErrorCode string         `json:"error_code,omitempty"`
}

func (p *CompiledParser) Parse(raw string) ParseResult {
	if p == nil || len(raw) > p.maxBytes {
		return abstain("record_limit")
	}
	if nestingDepth(raw) > MaxNesting {
		return abstain("nesting_limit")
	}
	values := map[string]any{}
	switch p.spec.Kind {
	case ParserRegex:
		loc := p.re.FindStringSubmatchIndex(raw)
		if loc == nil || loc[0] != 0 || loc[1] != len(raw) {
			return abstain("no_match")
		}
		for _, f := range p.spec.Fields {
			if loc[2*f.Capture] < 0 {
				continue
			}
			values[f.Name] = raw[loc[2*f.Capture]:loc[2*f.Capture+1]]
		}
	case ParserDelimited:
		fields, err := splitFieldsLimit(raw, []rune(p.spec.Delimiter)[0], MaxTokens)
		if err != nil {
			return abstain(errorCode(err))
		}
		if len(fields) > MaxTokens {
			return abstain("token_limit")
		}
		for _, f := range p.spec.Fields {
			if f.Index < len(fields) {
				values[f.Name] = fields[f.Index]
			}
		}
	case ParserJSON:
		var obj map[string]any
		if err := json.Unmarshal([]byte(raw), &obj); err != nil || obj == nil {
			return abstain("json_invalid")
		}
		if len(obj) > MaxTokens {
			return abstain("token_limit")
		}
		for _, f := range p.spec.Fields {
			if v, ok := obj[f.Source]; ok {
				values[f.Name] = v
			}
		}
	default:
		return abstain("kind_unsupported")
	}
	missing := make([]string, 0)
	requiredMissing := false
	for _, f := range p.spec.Fields {
		if _, ok := values[f.Name]; !ok {
			missing = append(missing, f.Name)
			requiredMissing = requiredMissing || f.Required
		}
	}
	sort.Strings(missing)
	if len(values) == 0 {
		return ParseResult{Outcome: OutcomeAbstain, Fields: values, Missing: missing, ErrorCode: "no_fields"}
	}
	if requiredMissing || len(missing) > 0 {
		return ParseResult{Outcome: OutcomePartial, Fields: values, Missing: missing}
	}
	return ParseResult{Outcome: OutcomeParsed, Fields: values}
}

func abstain(code string) ParseResult {
	return ParseResult{Outcome: OutcomeAbstain, Fields: map[string]any{}, ErrorCode: code}
}
func errorCode(err error) string {
	var ce *CompileError
	if errors.As(err, &ce) {
		return ce.Code
	}
	if err.Error() == "token_limit" || err.Error() == "record_limit" {
		return err.Error()
	}
	if strings.Contains(err.Error(), "quote") {
		return "quote_unclosed"
	}
	return "tokenize_invalid"
}
func safeID(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}
func cloneSpec(s ParserSpec) ParserSpec { s.Fields = append([]FieldSpec(nil), s.Fields...); return s }

// nestingDepth counts structural delimiters while ignoring escaped and quoted text.
func nestingDepth(s string) int {
	depth, maxDepth := 0, 0
	var quote rune
	escaped := false
	for _, r := range s {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		if r == '(' || r == '[' || r == '{' {
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
		} else if r == ')' || r == ']' || r == '}' {
			depth--
			if depth < 0 {
				return MaxNesting + 1
			}
		}
	}
	return maxDepth
}
func validRegexCaptures(pattern string, limit int) bool {
	count := 0
	escaped, inClass := false, false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '[' {
			inClass = true
			continue
		}
		if c == ']' {
			inClass = false
			continue
		}
		if !inClass && c == '(' && (i+1 >= len(pattern) || pattern[i+1] != '?') {
			count++
			if count > limit {
				return false
			}
		}
	}
	return true
}

// EvaluationDimensions keeps correctness axes separate and makes no probabilistic confidence claim.
type EvaluationDimensions struct {
	ExactMatchRate float64 `json:"exact_match_rate"`
	FieldPrecision float64 `json:"field_precision"`
	FieldRecall    float64 `json:"field_recall"`
	Ambiguous      bool    `json:"ambiguous"`
	ErrorCount     int     `json:"error_count"`
	Outcome        Outcome `json:"outcome"`
}

func Evaluate(expected, actual map[string]any, outcome Outcome, ambiguous bool, errors int) EvaluationDimensions {
	matched := 0
	for k, v := range actual {
		if e, ok := expected[k]; ok && fmt.Sprint(e) == fmt.Sprint(v) {
			matched++
		}
	}
	precision, recall := 0.0, 0.0
	if len(actual) > 0 {
		precision = float64(matched) / float64(len(actual))
	}
	if len(expected) > 0 {
		recall = float64(matched) / float64(len(expected))
	}
	exact := 0.0
	if len(expected) == len(actual) && matched == len(expected) {
		exact = 1
	}
	return EvaluationDimensions{ExactMatchRate: exact, FieldPrecision: precision, FieldRecall: recall, Ambiguous: ambiguous, ErrorCount: errors, Outcome: outcome}
}
