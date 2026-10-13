package report_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/GhandyP/executive-report-agent/internal/kpi"
	"github.com/GhandyP/executive-report-agent/internal/report"
)

// fixture is the facts of a two-period revenue series: 100 in Q2, 125 in Q3.
func fixture(t *testing.T) (kpi.Input, []kpi.Fact) {
	t.Helper()
	base, then := 100.0, 125.0
	in := kpi.Input{
		Period: "2026-Q3",
		Metrics: []kpi.Row{
			{ID: "revenue", Period: "2026-Q2", Value: &base, Unit: "USD", Source: "finance"},
			{ID: "revenue", Period: "2026-Q3", Value: &then, Unit: "USD", Source: "finance"},
		},
	}
	return in, kpi.ComputeFacts(in)
}

const supportedAnswer = `{
  "highlights": [
    {"text": "Revenue reached 125, up 25 from 100 in the previous period.", "evidenceIDs": ["revenue@2026-Q3"]}
  ],
  "trends": [
    {"text": "Revenue grew 25% against the prior period.", "evidenceIDs": ["revenue@2026-Q3"]}
  ],
  "risks": [
    {"text": "Revenue depends on a single source: finance.", "evidenceIDs": ["revenue@2026-Q3"]}
  ],
  "questions": [
    {"text": "Is the reporting period complete?", "evidenceIDs": ["revenue@2026-Q2"]}
  ]
}`

// S11: the prompt is the only channel between the deterministic facts and the
// model, so it must carry every fact id and every number exactly once.
func TestRenderPromptIsDeterministicAndCarriesTheFacts(t *testing.T) {
	in, facts := fixture(t)

	first := report.RenderPrompt(in, facts)
	second := report.RenderPrompt(in, facts)
	if first != second {
		t.Fatal("RenderPrompt is not deterministic")
	}
	for _, want := range []string{"2026-Q3", "revenue@2026-Q2", "revenue@2026-Q3", "100", "125", "25", "USD", "finance"} {
		if !strings.Contains(first, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}
}

// S12: a missing value and a zero base must reach the model as facts, not as
// instructions to invent them.
func TestRenderPromptShowsGapsAndZeroBases(t *testing.T) {
	base, then := 0.0, 50.0
	in := kpi.Input{
		Period: "2026-Q3",
		Metrics: []kpi.Row{
			{ID: "signups", Period: "2026-Q2", Value: &base, Unit: "count", Source: "ops"},
			{ID: "signups", Period: "2026-Q3", Value: &then, Unit: "count", Source: "ops"},
			{ID: "churn", Period: "2026-Q3", Value: nil, Unit: "count", Source: "ops"},
		},
	}

	prompt := report.RenderPrompt(in, kpi.ComputeFacts(in))

	for _, want := range []string{"no percentage change", "missing"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}
}

// S6, A2: a supported answer is accepted and stamped with the deterministic
// facts, which the model never supplied.
func TestParseNarrativeAcceptsASupportedAnswer(t *testing.T) {
	in, facts := fixture(t)

	got, err := report.ParseNarrative(in, facts, []byte(supportedAnswer))
	if err != nil {
		t.Fatalf("ParseNarrative: %v", err)
	}
	if got.Period != "2026-Q3" {
		t.Errorf("period = %q, want 2026-Q3", got.Period)
	}
	if len(got.Narrative.Highlights) != 1 || len(got.Narrative.Trends) != 1 {
		t.Errorf("narrative = %+v, want the supplied highlights and trends", got.Narrative)
	}
	if len(got.Facts) != len(facts) {
		t.Fatalf("facts = %d, want the %d computed facts", len(got.Facts), len(facts))
	}
	if got.Status != report.StatusDraft {
		t.Errorf("status = %q, want %q", got.Status, report.StatusDraft)
	}
}

// S9, A5: the report is a draft with no approver, because distribution needs an
// identified human.
func TestReportIsMarkedDraftWithoutAnApprover(t *testing.T) {
	in, facts := fixture(t)

	got, err := report.ParseNarrative(in, facts, []byte(supportedAnswer))
	if err != nil {
		t.Fatalf("ParseNarrative: %v", err)
	}
	if got.Status != report.StatusDraft {
		t.Fatalf("status = %q, want draft", got.Status)
	}
	if got.ApprovedBy != "" {
		t.Errorf("approvedBy = %q, want empty", got.ApprovedBy)
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if !strings.Contains(string(encoded), `"status":"draft"`) {
		t.Errorf("encoded report = %s, want a draft status", encoded)
	}
}

// S6, A2: a number the facts never produced rejects the answer.
func TestParseNarrativeRejectsAnUnsupportedNumber(t *testing.T) {
	in, facts := fixture(t)
	answer := `{"highlights":[{"text":"Revenue grew 42% this quarter.","evidenceIDs":["revenue@2026-Q3"]}],"trends":[],"risks":[],"questions":[]}`

	err := parseErr(t, in, facts, answer)
	if !errors.Is(err, report.ErrUnsupportedNumber) {
		t.Fatalf("error = %v, want ErrUnsupportedNumber", err)
	}
	if !strings.Contains(err.Error(), "42") {
		t.Errorf("error = %v, want it to name the unsupported number", err)
	}
}

// S6: a period label and its year are not unsupported numbers.
func TestParseNarrativeAcceptsPeriodLabelsAndYears(t *testing.T) {
	in, facts := fixture(t)
	answer := `{"highlights":[{"text":"In 2026-Q3 revenue reached 125, up 25 year over year within 2026.","evidenceIDs":["revenue@2026-Q3"]}],"trends":[],"risks":[],"questions":[]}`

	if _, err := report.ParseNarrative(in, facts, []byte(answer)); err != nil {
		t.Fatalf("ParseNarrative: %v, want the period label and year accepted", err)
	}
}

// S6: the accepted forms are the exact source value and a percentage rounded to
// two decimals.
func TestParseNarrativeAcceptsARoundedPercentage(t *testing.T) {
	base, then := 3.0, 4.0
	in := kpi.Input{
		Period: "2026-Q3",
		Metrics: []kpi.Row{
			{ID: "cost", Period: "2026-Q2", Value: &base, Unit: "USD", Source: "finance"},
			{ID: "cost", Period: "2026-Q3", Value: &then, Unit: "USD", Source: "finance"},
		},
	}
	facts := kpi.ComputeFacts(in)
	answer := `{"highlights":[{"text":"Cost rose 33.33% to 4.","evidenceIDs":["cost@2026-Q3"]}],"trends":[],"risks":[],"questions":[]}`

	if _, err := report.ParseNarrative(in, facts, []byte(answer)); err != nil {
		t.Fatalf("ParseNarrative: %v, want 33.33%% accepted as the rounded percentage", err)
	}
}

// S6: an item must point at the facts that support it.
func TestParseNarrativeRejectsAnUnknownCitation(t *testing.T) {
	in, facts := fixture(t)
	answer := `{"highlights":[{"text":"Revenue reached 125.","evidenceIDs":["revenue@2025-Q1"]}],"trends":[],"risks":[],"questions":[]}`

	err := parseErr(t, in, facts, answer)
	if !errors.Is(err, report.ErrUnknownCitation) {
		t.Fatalf("error = %v, want ErrUnknownCitation", err)
	}
	if !strings.Contains(err.Error(), "revenue@2025-Q1") {
		t.Errorf("error = %v, want it to name the unknown citation", err)
	}
}

func TestParseNarrativeRejectsAnItemWithoutEvidence(t *testing.T) {
	in, facts := fixture(t)
	answer := `{"highlights":[{"text":"Revenue reached 125.","evidenceIDs":[]}],"trends":[],"risks":[],"questions":[]}`

	err := parseErr(t, in, facts, answer)
	if !errors.Is(err, report.ErrMissingEvidence) {
		t.Fatalf("error = %v, want ErrMissingEvidence", err)
	}
	if !strings.Contains(err.Error(), "highlights[0]") {
		t.Errorf("error = %v, want it to name the offending item", err)
	}
}

func TestParseNarrativeRejectsAnEmptyAnswer(t *testing.T) {
	in, facts := fixture(t)
	answer := `{"highlights":[],"trends":[],"risks":[],"questions":[]}`

	err := parseErr(t, in, facts, answer)
	if !errors.Is(err, report.ErrNoNarrative) {
		t.Fatalf("error = %v, want ErrNoNarrative", err)
	}
}

func TestParseNarrativeRejectsMalformedJSON(t *testing.T) {
	in, facts := fixture(t)

	err := parseErr(t, in, facts, `{"highlights":[`)
	if !strings.Contains(err.Error(), "decode model answer") {
		t.Fatalf("error = %v, want a decode failure", err)
	}
}

// S6: the facts in the report keep the supplied digits even when the narrative
// rounds them.
func TestParseNarrativeKeepsSuppliedDigitsInTheFacts(t *testing.T) {
	base, then := 1234.5678, 987.65
	in := kpi.Input{
		Period: "2026-Q3",
		Metrics: []kpi.Row{
			{ID: "revenue", Period: "2026-Q2", Value: &base, Unit: "USD", Source: "finance"},
			{ID: "revenue", Period: "2026-Q3", Value: &then, Unit: "USD", Source: "finance"},
		},
	}
	facts := kpi.ComputeFacts(in)
	answer := `{"highlights":[{"text":"Revenue reached 987.65, down from 1234.5678.","evidenceIDs":["revenue@2026-Q3"]}],"trends":[],"risks":[],"questions":[]}`

	got, err := report.ParseNarrative(in, facts, []byte(answer))
	if err != nil {
		t.Fatalf("ParseNarrative: %v", err)
	}
	encoded, err := json.Marshal(got.Facts)
	if err != nil {
		t.Fatalf("marshal facts: %v", err)
	}
	for _, want := range []string{"1234.5678", "987.65"} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("facts = %s, want the supplied %s", encoded, want)
		}
	}
}

func parseErr(t *testing.T, in kpi.Input, facts []kpi.Fact, answer string) error {
	t.Helper()
	got, err := report.ParseNarrative(in, facts, []byte(answer))
	if err == nil {
		t.Fatalf("ParseNarrative accepted %s, want an error (report: %+v)", answer, got)
	}
	return err
}
