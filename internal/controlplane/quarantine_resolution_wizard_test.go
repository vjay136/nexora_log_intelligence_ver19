package controlplane

import (
	"context"
	"strings"
	"testing"

	parserpolicy "nexora.local/nli-v19/parser-policy"
)

func TestQuarantineResolutionWizardFixtureCompilesToExistingRuntime(t *testing.T) {
	const raw = "20250812 130035122+0100 aaa-gmailrgo009 mail 3609 0 140293768701696 Note;systemlogs_for_data"
	definition := map[string]any{
		"pattern_type": "regex",
		"pattern":      `^(?P<date>\S+)\s+(?P<time_timezone>\S+)\s+(?P<hostname>\S+)\s+(?P<service>\S+)\s+(?P<pid>\S+)\s+(?P<status>\S+)\s+(?P<minor_pid>\S+)\s+(?P<event_type>[^;\s]+);(?P<event_name>\S+)$`,
		"fields": []any{
			map[string]any{"name": "date", "source": "group:date", "type": "string"},
			map[string]any{"name": "time_timezone", "source": "group:time_timezone", "type": "string"},
			map[string]any{"name": "hostname", "source": "group:hostname", "type": "string"},
			map[string]any{"name": "service", "source": "group:service", "type": "string"},
			map[string]any{"name": "pid", "source": "group:pid", "type": "int"},
			map[string]any{"name": "status", "source": "group:status", "type": "int"},
			map[string]any{"name": "minor_pid", "source": "group:minor_pid", "type": "int"},
			map[string]any{"name": "event_type", "source": "group:event_type", "type": "string"},
			map[string]any{"name": "event_name", "source": "group:event_name", "type": "string"},
		},
	}
	pack, err := assuranceRuntimePack("qrw-mail", "0.1.0", "operator", definition, []assuranceRequest{{Raw: raw, Service: "mail"}})
	if err != nil {
		t.Fatalf("compile wizard candidate through existing runtime: %v", err)
	}
	executor, err := parserpolicy.NewParserPackExecutor(pack)
	if err != nil {
		t.Fatal(err)
	}
	result, err := executor.Execute(&parserpolicy.ExecutionRequest{Input: raw, Service: "mail", NodeID: "node-1", Context: context.Background()})
	if err != nil {
		t.Fatal(err)
	}
	if result.Quarantined {
		t.Fatalf("verified wizard fixture was quarantined: %+v", result)
	}
	wants := map[string]any{"date": "20250812", "time_timezone": "130035122+0100", "hostname": "aaa-gmailrgo009", "service": "mail", "pid": 3609, "status": 0, "minor_pid": 140293768701696, "event_type": "Note", "event_name": "systemlogs_for_data"}
	for key, want := range wants {
		got, ok := result.EventPayload.Get(key)
		if !ok || got != want {
			t.Fatalf("field %s = %#v present=%t, want %#v", key, got, ok, want)
		}
	}
}

func TestQuarantineResolutionWizardRejectsResourceExceedingCandidate(t *testing.T) {
	definition := map[string]any{
		"pattern_type": "regex",
		"pattern":      strings.Repeat("a", parserpolicy.MaxRegexLength+1),
		"fields":       []any{map[string]any{"name": "message", "source": "group:message", "type": "string"}},
	}
	if _, err := assuranceRuntimePack("unsafe-qrw", "0.1.0", "operator", definition, []assuranceRequest{{Raw: "a", Service: "mail"}}); err == nil {
		t.Fatal("resource-exceeding wizard candidate compiled into the runtime")
	}
}

func TestQuarantineResolutionWizardPreservesRuntimePresenceAndValidation(t *testing.T) {
	definition := map[string]any{
		"pattern_type": "regex",
		"pattern":      `^(?P<level>\w+)(?:\s+(?P<count>\d+))?$`,
		"fields": []any{
			map[string]any{"name": "level", "source": "group:level", "type": "string", "required": true, "validation": "one_of=INFO|WARN"},
			map[string]any{"name": "count", "source": "group:count", "type": "int", "required": false, "validation": "min=0,max=10"},
		},
	}
	pack, err := assuranceRuntimePack("qrw-schema", "0.1.0", "operator", definition, nil)
	if err != nil {
		t.Fatal(err)
	}
	level := pack.Rules[0].FieldSchema["level"]
	count := pack.Rules[0].FieldSchema["count"]
	if !level.Required || count.Required {
		t.Fatalf("presence policy changed during runtime conversion: level=%+v count=%+v", level, count)
	}
	if len(level.Enum) != 2 || level.Enum[0] != "INFO" || level.Enum[1] != "WARN" {
		t.Fatalf("enum validation changed during runtime conversion: %+v", level.Enum)
	}
	if count.Range == nil || count.Range.Min == nil || count.Range.Max == nil || *count.Range.Min != 0 || *count.Range.Max != 10 {
		t.Fatalf("range validation changed during runtime conversion: %+v", count.Range)
	}

	executor, err := parserpolicy.NewParserPackExecutor(pack)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"INFO", "WARN 10"} {
		result, runErr := executor.Execute(&parserpolicy.ExecutionRequest{Input: input, Service: "mail", NodeID: "node-1", Context: context.Background()})
		if runErr != nil || result.Quarantined {
			t.Fatalf("valid wizard input %q rejected: result=%+v err=%v", input, result, runErr)
		}
	}
	for _, input := range []string{"DEBUG 5", "INFO 11"} {
		result, runErr := executor.Execute(&parserpolicy.ExecutionRequest{Input: input, Service: "mail", NodeID: "node-1", Context: context.Background()})
		if runErr != nil {
			t.Fatalf("execute invalid wizard input %q: %v", input, runErr)
		}
		if !result.Quarantined {
			t.Fatalf("invalid wizard input %q bypassed runtime schema", input)
		}
	}
}

func TestQuarantineResolutionWizardRejectsPreviewOnlyValidation(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		fieldType  string
		validation string
	}{
		{name: "unsupported", fieldType: "string", validation: "min_length=2"},
		{name: "non-finite", fieldType: "float", validation: "min=NaN"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			definition := map[string]any{
				"pattern_type": "regex",
				"pattern":      `^(?P<message>.*)$`,
				"fields": []any{
					map[string]any{"name": "message", "source": "group:message", "type": testCase.fieldType, "validation": testCase.validation},
				},
			}
			if _, err := assuranceRuntimePack("qrw-preview-only", "0.1.0", "operator", definition, nil); err == nil {
				t.Fatalf("unsafe validation %q compiled into the runtime", testCase.validation)
			}
		})
	}
}
