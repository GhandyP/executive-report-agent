package kpi_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/GhandyP/executive-report-agent/internal/kpi"
)

func value(v float64) *float64 { return &v }

func row(id, period string, v *float64, unit, source string) kpi.Row {
	return kpi.Row{ID: id, Period: period, Value: v, Unit: unit, Source: source}
}

// S4, S5, A1: several periods per metric are the point of the contract, and a
// repeated period is allowed because it is a restatement, not a duplicate.
func TestValidateAcceptsMultiPeriodRowsAndRestatements(t *testing.T) {
	in := kpi.Input{
		Period: "2026-Q3",
		Metrics: []kpi.Row{
			row("revenue", "2026-Q2", value(100), "USD", "finance"),
			row("revenue", "2026-Q3", value(125), "USD", "finance"),
			row("revenue", "2026-Q3", value(124), "USD", "finance/restated"),
		},
	}
	if err := kpi.Validate(in); err != nil {
		t.Fatalf("Validate: %v, want nil", err)
	}
}

// S12: a missing value is valid input; it is shown later instead of being filled.
func TestValidateAcceptsAMissingValue(t *testing.T) {
	in := kpi.Input{
		Period:  "2026-Q3",
		Metrics: []kpi.Row{row("orders", "2026-Q3", nil, "count", "ops")},
	}
	if err := kpi.Validate(in); err != nil {
		t.Fatalf("Validate: %v, want nil", err)
	}
}

// S12: mixed units for one metric id are refused, never converted.
func TestValidateRejectsMixedUnits(t *testing.T) {
	in := kpi.Input{
		Period: "2026-Q3",
		Metrics: []kpi.Row{
			row("revenue", "2026-Q2", value(100), "USD", "finance"),
			row("revenue", "2026-Q3", value(90), "EUR", "finance"),
		},
	}
	err := kpi.Validate(in)
	if !errors.Is(err, kpi.ErrMixedUnits) {
		t.Fatalf("Validate = %v, want ErrMixedUnits", err)
	}
	for _, want := range []string{"revenue", "USD", "EUR"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to name %q", err, want)
		}
	}
}

func TestValidateRejectsIncompleteInput(t *testing.T) {
	tests := []struct {
		name     string
		in       kpi.Input
		want     error
		namesRow bool
	}{
		{
			name: "missing reporting period",
			in:   kpi.Input{Metrics: []kpi.Row{row("revenue", "2026-Q3", value(1), "USD", "finance")}},
			want: kpi.ErrMissingPeriod,
		},
		{
			name: "no metric rows",
			in:   kpi.Input{Period: "2026-Q3"},
			want: kpi.ErrNoMetrics,
		},
		{
			name:     "missing metric id",
			in:       kpi.Input{Period: "2026-Q3", Metrics: []kpi.Row{row("", "2026-Q3", value(1), "USD", "finance")}},
			want:     kpi.ErrMissingMetricID,
			namesRow: true,
		},
		{
			name:     "missing row period",
			in:       kpi.Input{Period: "2026-Q3", Metrics: []kpi.Row{row("revenue", "", value(1), "USD", "finance")}},
			want:     kpi.ErrMissingRowPeriod,
			namesRow: true,
		},
		{
			name:     "missing unit",
			in:       kpi.Input{Period: "2026-Q3", Metrics: []kpi.Row{row("revenue", "2026-Q3", value(1), "", "finance")}},
			want:     kpi.ErrMissingUnit,
			namesRow: true,
		},
		{
			name:     "missing source",
			in:       kpi.Input{Period: "2026-Q3", Metrics: []kpi.Row{row("revenue", "2026-Q3", value(1), "USD", "")}},
			want:     kpi.ErrMissingSource,
			namesRow: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := kpi.Validate(test.in)
			if !errors.Is(err, test.want) {
				t.Fatalf("Validate = %v, want %v", err, test.want)
			}
			if !test.namesRow {
				return
			}
			if !strings.Contains(err.Error(), "metric[0]") {
				t.Errorf("error = %v, want it to name the offending row", err)
			}
		})
	}
}

func TestNormalizeTrimsEverySuppliedField(t *testing.T) {
	in := kpi.Normalize(kpi.Input{
		Period:  " 2026-Q3 ",
		Metrics: []kpi.Row{row(" revenue ", " 2026-Q3 ", value(1), " USD ", " finance ")},
	})

	if in.Period != "2026-Q3" {
		t.Errorf("period = %q, want %q", in.Period, "2026-Q3")
	}
	got := in.Metrics[0]
	if got.ID != "revenue" || got.Unit != "USD" || got.Source != "finance" || got.Period != "2026-Q3" {
		t.Errorf("row = %+v, want every field trimmed", got)
	}
	if got.Value == nil || *got.Value != 1 {
		t.Errorf("value = %v, want 1", got.Value)
	}
}
