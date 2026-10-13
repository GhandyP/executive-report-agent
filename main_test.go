package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtures = "testdata"

// S6, A5, A6: a dry run validates the input and shows the deterministic facts
// and the exact model payload without credentials and without a model call.
func TestDryRunShowsFactsAndPayloadWithoutCredentials(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")
	input, err := os.Open(filepath.Join(fixtures, "kpis_ok.json"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer input.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{"-in", "-", "-dry-run"}, input, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitOK, stderr.String())
	}

	var facts []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &facts); err != nil {
		t.Fatalf("dry-run output is not a facts array: %v\n%s", err, stdout.String())
	}
	if len(facts) == 0 {
		t.Fatal("dry-run printed no facts")
	}
	encoded := stdout.String()
	for _, want := range []string{"1120000", "1050000", "0.39", "churn@2026-Q3"} {
		if !strings.Contains(encoded, want) {
			t.Errorf("facts are missing %q\n%s", want, encoded)
		}
	}
	for _, want := range []string{"reporting period: 2026-Q3", "revenue@2026-Q3", "value missing"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("model payload is missing %q\n%s", want, stderr.String())
		}
	}
}

// S7: a gap and a zero base are reported, not filled.
func TestWarningsNameGapsAndZeroBases(t *testing.T) {
	input, err := os.Open(filepath.Join(fixtures, "kpis_zero_base_and_gap.json"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer input.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{"-in", "-", "-dry-run"}, input, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitOK, stderr.String())
	}
	for _, want := range []string{"signups", "zero", "pipeline"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("payload does not mention %q\n%s", want, stderr.String())
		}
	}
}

// S12: mixed units are refused with the offending values named.
func TestInvalidInputExitsOneAndWritesNothing(t *testing.T) {
	out := filepath.Join(t.TempDir(), "report.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-in", filepath.Join(fixtures, "kpis_mixed_units.json"), "-out", out}, nil, &stdout, &stderr)

	if code != exitFailure {
		t.Fatalf("exit = %d, want %d", code, exitFailure)
	}
	for _, want := range []string{"USD", "EUR"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr = %q, want it to name %q", stderr.String(), want)
		}
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing", stdout.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("output file exists, want no file written (err = %v)", err)
	}
}

// A6: without credentials no model call happens and no report is written.
func TestMissingCredentialsExitOneWithoutWritingAReport(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")
	out := filepath.Join(t.TempDir(), "report.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-in", filepath.Join(fixtures, "kpis_ok.json"), "-out", out}, nil, &stdout, &stderr)

	if code != exitFailure {
		t.Fatalf("exit = %d, want %d", code, exitFailure)
	}
	if !strings.Contains(stderr.String(), "GOOGLE_API_KEY") {
		t.Errorf("stderr = %q, want it to name GOOGLE_API_KEY", stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("output file exists, want no file written (err = %v)", err)
	}
}

func TestUnknownInputFieldIsRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-in", "-", "-dry-run"}, strings.NewReader(`{"period":"2026-Q3","metrics":[],"total":1}`), &stdout, &stderr)

	if code != exitFailure {
		t.Fatalf("exit = %d, want %d", code, exitFailure)
	}
	if !strings.Contains(stderr.String(), "total") {
		t.Errorf("stderr = %q, want it to name the unknown field", stderr.String())
	}
}

func TestWrongCommandLineExitsTwo(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"-nope"}},
		{name: "unexpected argument", args: []string{"report.json"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(test.args, nil, &stdout, &stderr); code != exitUsage {
				t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitUsage, stderr.String())
			}
		})
	}
}

func TestHelpExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := run([]string{"-h"}, nil, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit = %d, want %d", code, exitOK)
	}
	for _, want := range []string{"-dry-run", "GOOGLE_API_KEY", "Exit codes", "draft"} {
		combined := stdout.String() + stderr.String()
		if !strings.Contains(combined, want) {
			t.Errorf("usage is missing %q", want)
		}
	}
}

// S6: the numbers a report carries are the supplied digits, and a restatement
// stays readable next to the value that replaced it.
func TestFactsPreserveSuppliedDigits(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-in", filepath.Join(fixtures, "kpis_restated.json"), "-dry-run"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitOK, stderr.String())
	}

	var facts []struct {
		ID       string   `json:"id"`
		Value    *float64 `json:"value"`
		Restated bool     `json:"restated"`
		Note     string   `json:"note"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &facts); err != nil {
		t.Fatalf("dry-run output is not a facts array: %v\n%s", err, stdout.String())
	}

	byID := make(map[string]int, len(facts))
	for i, fact := range facts {
		byID[fact.ID] = i
	}
	idx, ok := byID["revenue@2026-Q3"]
	if !ok {
		t.Fatalf("no revenue@2026-Q3 fact in %v", byID)
	}
	got := facts[idx]
	if got.Value == nil || *got.Value != 124 {
		t.Errorf("value = %v, want the restated 124", got.Value)
	}
	if !got.Restated {
		t.Error("restated = false, want true")
	}
	if !strings.Contains(got.Note, "100") {
		t.Errorf("note = %q, want the earlier value 100 to stay readable", got.Note)
	}
}
