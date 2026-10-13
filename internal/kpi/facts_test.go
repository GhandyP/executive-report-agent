package kpi_test

import (
	"math"
	"testing"

	"github.com/GhandyP/executive-report-agent/internal/kpi"
)

// factsFor builds an input whose rows are supplied as (id, period, value).
type suppliedRow struct {
	id     string
	period string
	value  *float64
}

func factsFor(t *testing.T, period string, rows ...suppliedRow) []kpi.Fact {
	t.Helper()
	in := kpi.Input{Period: period}
	for _, row := range rows {
		in.Metrics = append(in.Metrics, kpi.Row{
			ID:     row.id,
			Period: row.period,
			Value:  row.value,
			Unit:   "USD",
			Source: "finance",
		})
	}
	return kpi.ComputeFacts(in)
}

func fact(t *testing.T, facts []kpi.Fact, id, period string) kpi.Fact {
	t.Helper()
	for _, f := range facts {
		if f.MetricID == id && f.Period == period {
			return f
		}
	}
	t.Fatalf("no fact for %s@%s in %d facts", id, period, len(facts))
	return kpi.Fact{}
}

// S5, S6: one fact per metric per period, with the source value unchanged and
// the change computed against the previous available period.
func TestComputeFactsComputesChangeAndPercentPerMetric(t *testing.T) {
	facts := factsFor(t, "2026-Q3",
		suppliedRow{"revenue", "2026-Q2", value(100)},
		suppliedRow{"revenue", "2026-Q3", value(125)},
		suppliedRow{"orders", "2026-Q3", value(40)},
	)

	if len(facts) != 3 {
		t.Fatalf("facts = %d, want 3 (revenue for two periods, orders for one)", len(facts))
	}

	rev := fact(t, facts, "revenue", "2026-Q3")
	if rev.Value == nil || *rev.Value != 125 {
		t.Fatalf("revenue value = %v, want the supplied 125", rev.Value)
	}
	if rev.Change == nil || *rev.Change != 25 {
		t.Errorf("revenue change = %v, want 25", rev.Change)
	}
	if rev.ChangePercent == nil || *rev.ChangePercent != 25 {
		t.Errorf("revenue change percent = %v, want 25", rev.ChangePercent)
	}
	if rev.PreviousPeriod != "2026-Q2" {
		t.Errorf("revenue previous period = %q, want 2026-Q2", rev.PreviousPeriod)
	}

	orders := fact(t, facts, "orders", "2026-Q3")
	if orders.Change != nil || orders.ChangePercent != nil {
		t.Errorf("orders change = %v/%v, want both nil: no earlier period was supplied", orders.Change, orders.ChangePercent)
	}
}

// S12: a negative change keeps its sign.
func TestComputeFactsKeepsNegativeChanges(t *testing.T) {
	facts := factsFor(t, "2026-Q3",
		suppliedRow{"margin", "2026-Q2", value(125)},
		suppliedRow{"margin", "2026-Q3", value(100)},
	)

	got := fact(t, facts, "margin", "2026-Q3")
	if got.Change == nil || *got.Change != -25 {
		t.Fatalf("change = %v, want -25", got.Change)
	}
	if got.ChangePercent == nil || *got.ChangePercent != -20 {
		t.Errorf("change percent = %v, want -20", got.ChangePercent)
	}
}

// S7: a zero denominator is visible instead of an invented percentage.
func TestComputeFactsReportsAZeroBaseWithoutAPercentage(t *testing.T) {
	facts := factsFor(t, "2026-Q3",
		suppliedRow{"signups", "2026-Q2", value(0)},
		suppliedRow{"signups", "2026-Q3", value(50)},
	)

	got := fact(t, facts, "signups", "2026-Q3")
	if got.Change == nil || *got.Change != 50 {
		t.Fatalf("change = %v, want 50", got.Change)
	}
	if got.ChangePercent != nil {
		t.Errorf("change percent = %v, want nil: the base is zero", *got.ChangePercent)
	}
	if !got.ZeroBase {
		t.Error("zeroBase = false, want true so the report can explain the gap")
	}
}

// S7: a period supplied with a null value stays visible and computes nothing.
func TestComputeFactsKeepsMissingValuesVisible(t *testing.T) {
	facts := factsFor(t, "2026-Q3",
		suppliedRow{"churn", "2026-Q2", value(4)},
		suppliedRow{"churn", "2026-Q3", nil},
	)

	got := fact(t, facts, "churn", "2026-Q3")
	if !got.Missing {
		t.Error("missing = false, want true")
	}
	if got.Value != nil {
		t.Errorf("value = %v, want nil", *got.Value)
	}
	if got.Change != nil || got.ChangePercent != nil {
		t.Errorf("change = %v/%v, want both nil for a missing value", got.Change, got.ChangePercent)
	}
}

// S7: a metric that never supplied the reporting period still appears, as a gap.
func TestComputeFactsAddsTheReportingPeriodAsAMissingFact(t *testing.T) {
	facts := factsFor(t, "2026-Q3",
		suppliedRow{"pipeline", "2026-Q1", value(10)},
		suppliedRow{"pipeline", "2026-Q2", value(20)},
	)

	got := fact(t, facts, "pipeline", "2026-Q3")
	if !got.Missing {
		t.Fatal("missing = false, want a visible gap for the reporting period")
	}
}

// S7, S12: the base skips missing periods instead of treating them as zero.
func TestComputeFactsSkipsMissingPeriodsWhenChoosingTheBase(t *testing.T) {
	facts := factsFor(t, "2026-Q3",
		suppliedRow{"revenue", "2026-Q1", value(100)},
		suppliedRow{"revenue", "2026-Q2", nil},
		suppliedRow{"revenue", "2026-Q3", value(125)},
	)

	got := fact(t, facts, "revenue", "2026-Q3")
	if got.PreviousPeriod != "2026-Q1" {
		t.Fatalf("previous period = %q, want 2026-Q1: 2026-Q2 has no value", got.PreviousPeriod)
	}
	if got.Change == nil || *got.Change != 25 {
		t.Errorf("change = %v, want 25 against the last available value", got.Change)
	}
}

// S8, S12: a repeated period is a restatement. The last value wins and the
// earlier value stays readable on the fact.
func TestComputeFactsKeepsTheLastRestatementAndShowsTheEarlierValue(t *testing.T) {
	facts := factsFor(t, "2026-Q3",
		suppliedRow{"revenue", "2026-Q2", value(100)},
		suppliedRow{"revenue", "2026-Q3", value(100)},
		suppliedRow{"revenue", "2026-Q3", value(124)},
	)

	got := fact(t, facts, "revenue", "2026-Q3")
	if got.Value == nil || *got.Value != 124 {
		t.Fatalf("value = %v, want the restated 124", got.Value)
	}
	if !got.Restated {
		t.Error("restated = false, want true")
	}
	const wantNote = "restated: 2026-Q3 was supplied as 100 and restated as 124"
	if got.Note != wantNote {
		t.Errorf("note = %q, want %q", got.Note, wantNote)
	}
	if got.Change == nil || *got.Change != 24 {
		t.Errorf("change = %v, want 24 against the restated value", got.Change)
	}
}

// S6: a non-finite percentage is not a reported number.
func TestComputeFactsDropsANonFinitePercentage(t *testing.T) {
	facts := factsFor(t, "2026-Q3",
		suppliedRow{"ratio", "2026-Q2", value(1e-300)},
		suppliedRow{"ratio", "2026-Q3", value(1e300)},
	)

	got := fact(t, facts, "ratio", "2026-Q3")
	if got.ChangePercent != nil {
		t.Fatalf("change percent = %v, want nil for a non-finite result", *got.ChangePercent)
	}
	if got.Note == "" {
		t.Error("note is empty, want an explanation of the missing percentage")
	}
}

// S6: the canonical form of a number is the supplied digits, and negative zero
// is not a source value.
func TestCanonicalNumberPreservesSuppliedDigits(t *testing.T) {
	tests := map[float64]string{
		100:     "100",
		100.5:   "100.5",
		-0.0:    "0",
		0.1:     "0.1",
		1.0 / 3: "0.3333333333333333",
	}
	for in, want := range tests {
		if got := kpi.CanonicalNumber(in); got != want {
			t.Errorf("CanonicalNumber(%v) = %q, want %q", in, got, want)
		}
	}
	if got := kpi.CanonicalNumber(math.Copysign(0, -1)); got != "0" {
		t.Errorf("CanonicalNumber(-0) = %q, want %q", got, "0")
	}
}

// S6: the number index accepts an exact source value, an exact change and a
// percentage rounded to two decimals, and it names the fact that justifies each.
func TestNumberIndexAcceptsSourceValuesChangesAndRoundedPercentages(t *testing.T) {
	facts := factsFor(t, "2026-Q3",
		suppliedRow{"revenue", "2026-Q2", value(100)},
		suppliedRow{"revenue", "2026-Q3", value(133.5)},
	)

	index := kpi.NumberIndex(facts)

	for _, want := range []string{"100", "133.5", "33.5"} {
		if _, ok := index[want]; !ok {
			t.Errorf("index is missing %q (accepted numbers: %v)", want, keysOf(index))
		}
	}
	if _, ok := index["33.5"]; ok {
		if got := index["33.5"]; len(got) == 0 || got[0] != "revenue@2026-Q3" {
			t.Errorf("index[33.5] = %v, want it to name revenue@2026-Q3", got)
		}
	}
	if _, ok := index["7.25"]; ok {
		t.Error("index accepted 7.25, which no source value or calculation produced")
	}
}

func keysOf(index map[string][]string) []string {
	keys := make([]string, 0, len(index))
	for key := range index {
		keys = append(keys, key)
	}
	return keys
}

// S6: every supplied value survives into the index, whatever its shape.
func TestNumberIndexKeepsEverySuppliedValue(t *testing.T) {
	facts := factsFor(t, "2026-Q3",
		suppliedRow{"revenue", "2026-Q1", value(1234.5678)},
		suppliedRow{"revenue", "2026-Q2", value(-987.65)},
		suppliedRow{"orders", "2026-Q2", value(0)},
	)

	index := kpi.NumberIndex(facts)
	for _, want := range []string{"1234.5678", "-987.65", "0"} {
		if _, ok := index[want]; !ok {
			t.Errorf("index is missing the supplied value %q", want)
		}
	}
}
