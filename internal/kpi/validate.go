package kpi

import (
	"errors"
	"fmt"
	"strings"
)

// Contract violations reported by Validate. Each is a distinct error value so
// a caller can distinguish a missing field from a mixed-unit contradiction.
var (
	ErrMissingPeriod    = errors.New("period is required")
	ErrNoMetrics        = errors.New("at least one metric row is required")
	ErrMissingMetricID  = errors.New("id is required")
	ErrMissingRowPeriod = errors.New("period is required")
	ErrMissingUnit      = errors.New("unit is required")
	ErrMissingSource    = errors.New("source is required")
	ErrMixedUnits       = errors.New("the same metric is reported in more than one unit")
)

// Validate checks the contract rules that do not depend on a model:
//
//   - a reporting period and at least one row must be present;
//   - every row states an id, a period, a unit and a source;
//   - one metric id keeps one unit, because converting units here would change
//     a supplied number and the report must never do that.
//
// A missing value, a repeated period (a restatement) and several periods for
// one metric are all valid: they are handled visibly by ComputeFacts.
func Validate(in Input) error {
	if strings.TrimSpace(in.Period) == "" {
		return fmt.Errorf("input: %w", ErrMissingPeriod)
	}
	if len(in.Metrics) == 0 {
		return fmt.Errorf("input: %w", ErrNoMetrics)
	}

	units := make(map[string]string, len(in.Metrics))
	for i, row := range in.Metrics {
		id := strings.TrimSpace(row.ID)
		where := fmt.Sprintf("metric[%d]", i)
		if id != "" {
			where = fmt.Sprintf("metric[%d] %q", i, id)
		}

		switch {
		case id == "":
			return fmt.Errorf("%s: %w", where, ErrMissingMetricID)
		case strings.TrimSpace(row.Period) == "":
			return fmt.Errorf("%s: %w", where, ErrMissingRowPeriod)
		case strings.TrimSpace(row.Unit) == "":
			return fmt.Errorf("%s: %w", where, ErrMissingUnit)
		case strings.TrimSpace(row.Source) == "":
			return fmt.Errorf("%s: %w", where, ErrMissingSource)
		}

		unit := strings.TrimSpace(row.Unit)
		if known, ok := units[id]; ok && known != unit {
			return fmt.Errorf("%s: %w (%s, %s)", where, ErrMixedUnits, known, unit)
		}
		units[id] = unit
	}
	return nil
}
