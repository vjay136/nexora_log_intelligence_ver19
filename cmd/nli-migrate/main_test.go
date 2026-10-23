package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadBoundedAndAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(input, []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	if data, err := readBounded(input, 6); err != nil || string(data) != "12345" {
		t.Fatalf("readBounded data=%q err=%v", data, err)
	}
	if _, err := readBounded(input, 5); err == nil {
		t.Fatal("expected bounded-read error")
	}
	output := filepath.Join(dir, "nested", "report.json")
	if err := atomicWrite(output, []byte("result")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(output); err != nil || string(data) != "result" {
		t.Fatalf("atomic output=%q err=%v", data, err)
	}
}

func TestLoadRegressionsEnforcesIdentityAndSchema(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "regressions.json")
	valid := `{"source_file":"input.log","cases":[{"name":"one","input":"a","expected_payload":{"message":"a"},"expected_parser":"test"}]}`
	if err := os.WriteFile(fixture, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	cases, err := loadRegressions(fixture, "input.log")
	if err != nil || len(cases) != 1 {
		t.Fatalf("cases=%#v err=%v", cases, err)
	}
	if _, err := loadRegressions(fixture, "other.log"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("identity error = %v", err)
	}
	if err := os.WriteFile(fixture, []byte(`{"unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRegressions(fixture, "input.log"); err == nil {
		t.Fatal("expected unknown-field error")
	}
}

func TestRunRejectsMissingInputAndInvalidLimits(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(nil, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "-input is required") {
		t.Fatalf("missing-input error = %v", err)
	}
	if err := run([]string{"-input", "unused", "-max-files", "0"}, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "resource limits") {
		t.Fatalf("resource-limit error = %v", err)
	}
}
