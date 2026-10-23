package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSortInt64(t *testing.T) {
	values := []int64{5, -1, 5, 0, 2}
	sortInt64(values)
	want := []int64{-1, 0, 2, 5, 5}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("sorted values = %#v, want %#v", values, want)
	}
}

func TestRunBenchAccountsForNonEmptyInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.log")
	data := []byte("{\"level\":\"INFO\",\"message\":\"one\"}\n\n{\"level\":\"WARN\",\"message\":\"two\"}\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := runBench(path, "automatic")
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalLines != 2 || report.JSONValidity != 100 {
		t.Fatalf("benchmark accounting = %#v", report)
	}
	if report.Timestamp == "" || report.EngineVersion == "" || report.ParserCoverage < 1 || report.MemoryMB < 0 {
		t.Fatalf("incomplete benchmark report = %#v", report)
	}
	if report.MetricScope == "" {
		t.Fatal("benchmark must state its accuracy scope")
	}
}

func TestRunBenchRejectsMissingFile(t *testing.T) {
	if _, err := runBench(filepath.Join(t.TempDir(), "missing.log"), "automatic"); err == nil {
		t.Fatal("expected missing input error")
	}
}
