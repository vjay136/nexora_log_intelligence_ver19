package controlplane

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"nexora.local/nli-v19/parser-policy"
)

// assuranceRuntimePack converts the bounded Assurance regex definition into
// the strict IR consumed by the client runtime. The conversion is deliberately
// narrow: unsupported declarative forms stay test-only instead of being
// silently interpreted differently in production.
func assuranceRuntimePack(id, version, author string, definition map[string]any, cases []assuranceRequest) (*parserpolicy.ParserPack, error) {
	if definition["pattern_type"] != "regex" {
		return nil, fmt.Errorf("only validated regex candidates can be published to runtime")
	}
	pattern, _ := definition["pattern"].(string)
	compiled, err := regexp.Compile(pattern)
	if err != nil || pattern == "" || len(pattern) > parserpolicy.MaxRegexLength {
		return nil, fmt.Errorf("runtime regex is invalid or exceeds its size limit")
	}
	fields, ok := definition["fields"].([]any)
	if !ok || len(fields) == 0 {
		return nil, fmt.Errorf("runtime parser needs at least one mapped field")
	}
	groups := map[string]string{}
	schema := map[string]parserpolicy.FieldDef{}
	for _, item := range fields {
		field, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("runtime field mapping is invalid")
		}
		name, _ := field["name"].(string)
		source, _ := field["source"].(string)
		group := strings.TrimPrefix(source, "group:")
		if name == "" || group == source || !containsString(compiled.SubexpNames(), group) {
			return nil, fmt.Errorf("field %q must map to a named regex capture group", name)
		}
		groups[name] = group
		fieldType := parserpolicy.FieldTypeString
		if value, ok := field["type"].(string); ok && value != "" {
			fieldType = parserpolicy.FieldType(value)
		}
		schema[name] = parserpolicy.FieldDef{Type: fieldType}
	}
	service := ""
	if len(cases) != 0 {
		service = strings.TrimSpace(cases[0].Service)
	}
	if service == "" || service == "auto" {
		service = "*"
	}
	pack := &parserpolicy.ParserPack{
		ID: id, Version: version, Revision: 1, SourceType: "manual", Author: author,
		ReviewState: parserpolicy.ReviewStateApproved, CreatedAt: time.Now().UTC(),
		ResourceLimits: parserpolicy.DefaultResourceLimits(),
		Rules: []parserpolicy.ParserRule{{
			ID: id + "-rule", Name: id, Priority: 1, ServiceSelectors: []string{service},
			Stages:        []parserpolicy.Stage{{ID: id + "-regex", Name: "Assurance regex", Type: parserpolicy.StageTypeRegex, Config: map[string]any{"pattern": pattern, "groups": groups}, FailurePolicy: parserpolicy.FailurePolicyRejectRule}},
			FailurePolicy: parserpolicy.FailurePolicyQuarantine, FieldSchema: schema,
			ProjectionMode: parserpolicy.ProjectionModeAllowlist, AllowedFields: sortedKeys(schema), CountPolicy: parserpolicy.CountPolicyExactSource,
		}},
	}
	if err := parserpolicy.SealParserPack(pack); err != nil {
		return nil, err
	}
	if result := parserpolicy.ValidateParserPack(pack); !result.Valid {
		return nil, fmt.Errorf("runtime parser validation failed: %s", strings.Join(result.Errors, "; "))
	}
	encoded, err := json.Marshal(pack)
	if err != nil {
		return nil, err
	}
	if _, err := parserpolicy.LoadParserPackBytes(encoded); err != nil {
		return nil, err
	}
	return pack, nil
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func sortedKeys(fields map[string]parserpolicy.FieldDef) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	// Keep rule serialization and bundle hashes deterministic.
	sort.Strings(keys)
	return keys
}
