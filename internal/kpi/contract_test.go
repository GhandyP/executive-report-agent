package kpi_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GhandyP/executive-report-agent/internal/kpi"
)

// S4, S5: the contract carries the reporting period plus one row per supplied
// metric value, and a row with a null value stays in the input as a visible gap.
func TestLoadReadsTheDocumentedContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kpis.json")
	body := `{
		"period": "2026-Q3",
		"metrics": [
			{"id": "revenue", "period": "2026-Q2", "value": 100.5, "unit": "USD", "source": "finance/2026-09.csv"},
			{"id": "revenue", "period": "2026-Q3", "value": null, "unit": "USD", "source": "finance/2026-09.csv"}
		]
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	in, err := kpi.Load(path, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if in.Period != "2026-Q3" {
		t.Errorf("period = %q, want %q", in.Period, "2026-Q3")
	}
	if len(in.Metrics) != 2 {
		t.Fatalf("rows = %d, want 2", len(in.Metrics))
	}
	first := in.Metrics[0]
	if first.ID != "revenue" || first.Period != "2026-Q2" || first.Unit != "USD" || first.Source != "finance/2026-09.csv" {
		t.Errorf("first row = %+v, want the supplied revenue row", first)
	}
	if first.Value == nil || *first.Value != 100.5 {
		t.Errorf("first value = %v, want 100.5", first.Value)
	}
	if in.Metrics[1].Value != nil {
		t.Errorf("null value = %v, want nil so the gap stays visible", *in.Metrics[1].Value)
	}
}

func TestLoadReadsStdinWhenPathIsDash(t *testing.T) {
	in, err := kpi.Load("-", strings.NewReader(`{"period":"2026-Q3","metrics":[{"id":"orders","period":"2026-Q3","value":7,"unit":"count","source":"ops"}]}`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(in.Metrics) != 1 || in.Metrics[0].ID != "orders" {
		t.Fatalf("rows = %+v, want the supplied orders row", in.Metrics)
	}
}

// A typo in a field name must fail loudly instead of silently dropping data.
func TestLoadRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kpis.json")
	if err := os.WriteFile(path, []byte(`{"period":"2026-Q3","metrics":[],"totals":1}`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := kpi.Load(path, nil); err == nil {
		t.Fatal("Load accepted an unknown field, want an error naming it")
	} else if !strings.Contains(err.Error(), "totals") {
		t.Errorf("error = %v, want it to name the unknown field", err)
	}
}

func TestLoadRejectsMalformedJSON(t *testing.T) {
	if _, err := kpi.Load("-", strings.NewReader(`{"period":`)); err == nil {
		t.Fatal("Load accepted malformed JSON, want an error")
	}
}

func TestLoadReportsAMissingFile(t *testing.T) {
	if _, err := kpi.Load(filepath.Join(t.TempDir(), "absent.json"), nil); err == nil {
		t.Fatal("Load accepted a missing file, want an error")
	}
}
