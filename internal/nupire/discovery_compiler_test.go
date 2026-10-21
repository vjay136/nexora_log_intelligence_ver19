package nupire

import (
	"reflect"
	"strings"
	"testing"
)

func TestClassifyAndDelimiterInference(t *testing.T) {
	cases := []struct {
		line string
		want Structure
	}{{`{"x":1}`, StructureJSON}, {`<34>1 2026-01-01T12:00:00Z host app 1 ID - hello`, StructureSyslog}, {`127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET / HTTP/1.1" 200 12`, StructureAccess}, {`a=1 b="two words"`, StructureKV}, {`2026-01-01T12:00:00Z INFO ready`, StructureTimestamped}, {`hello world`, StructureUnknown}}
	for _, tc := range cases {
		if got := Classify(tc.line); got.Structure != tc.want || len(got.Evidence) == 0 {
			t.Errorf("Classify(%q)=%+v want %s", tc.line, got, tc.want)
		}
	}
	d, score, ok := InferDelimiter([]string{`a,"b,c",d`, `x,"y,z",q`})
	if !ok || d != ',' || score != 1 {
		t.Fatalf("delimiter=%q score=%v ok=%v", d, score, ok)
	}
	f, err := SplitFields(`a,"b,c",d`, ',')
	if err != nil || !reflect.DeepEqual(f, []string{"a", "b,c", "d"}) {
		t.Fatalf("fields=%q err=%v", f, err)
	}
	if _, err = SplitFields(`a,"unterminated`, ','); err == nil {
		t.Fatal("accepted unterminated quote")
	}
	if _, err = SplitFields(strings.Repeat("x,", MaxTokens), ','); err == nil || err.Error() != "token_limit" {
		t.Fatalf("unbounded token count error=%v", err)
	}
}

func TestDeclarativeCompilerAndOutcomes(t *testing.T) {
	spec := ParserSpec{Version: 1, ID: "access-v1", Kind: ParserRegex, Pattern: `^(\w+) (\d+)$`, Fields: []FieldSpec{{Name: "user", Capture: 1, Required: true}, {Name: "count", Capture: 2, Required: true}}}
	p, err := Compile(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Digest()) != 64 {
		t.Fatalf("digest=%q", p.Digest())
	}
	if got := p.Parse("alice 12"); got.Outcome != OutcomeParsed || got.Fields["user"] != "alice" {
		t.Fatalf("parse=%+v", got)
	}
	if got := p.Parse("alice"); got.Outcome != OutcomeAbstain {
		t.Fatalf("expected abstain: %+v", got)
	}
	delim, err := Compile(ParserSpec{Version: 1, ID: "csv", Kind: ParserDelimited, Delimiter: ",", Fields: []FieldSpec{{Name: "a", Index: 0, Required: true}, {Name: "b", Index: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := delim.Parse(`one`); got.Outcome != OutcomePartial || len(got.Missing) != 1 {
		t.Fatalf("partial=%+v", got)
	}
	js, err := Compile(ParserSpec{Version: 1, ID: "json", Kind: ParserJSON, Fields: []FieldSpec{{Name: "message", Source: "msg", Required: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := js.Parse(`{"other":true}`); got.Outcome != OutcomeAbstain {
		t.Fatalf("no mapped json fields must abstain: %+v", got)
	}
	if got := js.Parse(strings.Repeat("{", MaxNesting+1)); got.ErrorCode != "nesting_limit" {
		t.Fatalf("limit result=%+v", got)
	}
}

func TestCompilerRejectsComplexityAndUnsupportedKindsDeterministically(t *testing.T) {
	cases := []struct {
		spec ParserSpec
		code string
	}{{ParserSpec{Version: 1, ID: "bad", Kind: ParserRegex, Pattern: "[", Fields: []FieldSpec{{Name: "v", Capture: 1}}}, "pattern_syntax"}, {ParserSpec{Version: 1, ID: "bad", Kind: "code", Fields: []FieldSpec{{Name: "v"}}}, "kind_unsupported"}, {ParserSpec{Version: 1, ID: "bad", Kind: ParserRegex, Pattern: strings.Repeat("a", MaxPatternBytes+1), Fields: []FieldSpec{{Name: "v", Capture: 1}}}, "pattern_limit"}}
	for _, tc := range cases {
		for i := 0; i < 2; i++ {
			_, err := Compile(tc.spec)
			ce, ok := err.(*CompileError)
			if !ok || ce.Code != tc.code || err.Error() != tc.code {
				t.Fatalf("want deterministic %s, got %v", tc.code, err)
			}
		}
	}
}

func TestEvaluationDimensionsRemainSeparate(t *testing.T) {
	e := Evaluate(map[string]any{"a": 1, "b": 2}, map[string]any{"a": 1}, OutcomePartial, true, 3)
	if e.ExactMatchRate != 0 || e.FieldPrecision != 1 || e.FieldRecall != .5 || !e.Ambiguous || e.ErrorCount != 3 || e.Outcome != OutcomePartial {
		t.Fatalf("dimensions merged or wrong: %+v", e)
	}
}

func FuzzSplitFieldsNeverPanics(f *testing.F) {
	for _, s := range []string{"a,b", "\"a,b\",c", "a\\,b,c", "", `x,"open`} {
		f.Add(s, byte(','))
	}
	f.Fuzz(func(t *testing.T, input string, delimiter byte) {
		if delimiter == 0 {
			return
		}
		a, err := SplitFields(input, rune(delimiter))
		b, err2 := SplitFields(input, rune(delimiter))
		if err == nil && (err2 != nil || !reflect.DeepEqual(a, b)) {
			t.Fatal("tokenization is not deterministic")
		}
	})
}
