package kpi

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Fact is one deterministic statement about one metric in one period: either a
// supplied value or a visible gap, plus the change computed against the
// previous available period of the same metric.
//
// Every number on a Fact is a source value or a calculation over source values.
// No model is involved in producing them, which is what lets the report claim
// that each printed number is traceable.
type Fact struct {
	// ID is "<metricId>@<period>": the citation a narrative item carries.
	ID             string   `json:"id"`
	MetricID       string   `json:"metricId"`
	Unit           string   `json:"unit"`
	Source         string   `json:"source"`
	Period         string   `json:"period"`
	Value          *float64 `json:"value"`
	PreviousPeriod string   `json:"previousPeriod,omitempty"`
	PreviousValue  *float64 `json:"previousValue,omitempty"`
	Change         *float64 `json:"change,omitempty"`
	ChangePercent  *float64 `json:"changePercent,omitempty"`
	// ZeroBase marks a period whose base is zero, so no percentage exists.
	ZeroBase bool `json:"zeroBase,omitempty"`
	// Missing marks a period with no supplied value.
	Missing bool `json:"missing,omitempty"`
	// Restated marks a period that was supplied more than once.
	Restated bool   `json:"restated,omitempty"`
	Note     string `json:"note,omitempty"`
}

// ComputeFacts turns the supplied rows into one fact per metric and period.
//
// The periods of a metric are the ones it supplied plus the reporting period,
// so a metric that never supplied the reporting period still appears as a
// visible gap. Within a metric, a period supplied more than once keeps its last
// value and records the restatement. The change of a period is computed against
// the last earlier period that has a value, and a zero base reports no
// percentage instead of substituting one.
func ComputeFacts(in Input) []Fact {
	var facts []Fact

	for _, metricID := range metricOrder(in) {
		rows := rowsOf(in, metricID)
		unit, source := rows[0].Unit, rows[0].Source

		chosen := make(map[string]Row, len(rows))
		restatedFrom := make(map[string]*float64, len(rows))
		var periods []string
		for _, row := range rows {
			period := row.Period
			if _, known := chosen[period]; !known {
				periods = append(periods, period)
			} else {
				restatedFrom[period] = chosen[period].Value
			}
			chosen[period] = row
		}
		if _, ok := chosen[in.Period]; !ok {
			periods = append(periods, in.Period)
		}

		var previousPeriod string
		var previousValue *float64
		for _, period := range periods {
			f := Fact{
				ID:       metricID + "@" + period,
				MetricID: metricID,
				Unit:     unit,
				Source:   source,
				Period:   period,
			}

			row, supplied := chosen[period]
			if supplied {
				f.Unit = row.Unit
				f.Source = row.Source
				f.Value = row.Value
				if earlier, ok := restatedFrom[period]; ok {
					f.Restated = true
					f.Note = restatementNote(period, earlier, row.Value)
				}
			}

			if f.Value == nil {
				f.Missing = true
			} else {
				if previousValue != nil {
					change := *f.Value - *previousValue
					f.PreviousPeriod = previousPeriod
					f.PreviousValue = previousValue
					f.Change = &change
					switch percent, ok := percentChange(*previousValue, *f.Value); {
					case ok:
						f.ChangePercent = &percent
					case *previousValue == 0:
						f.ZeroBase = true
						f.Note = "no percentage change: the previous value is zero"
					default:
						f.Note = "no percentage change: the computed value is not a finite number"
					}
				}
				value := *f.Value
				period := period
				previousPeriod, previousValue = period, &value
			}

			facts = append(facts, f)
		}
	}
	return facts
}

// CanonicalNumber is the plain decimal form used for every fact number and for
// the number index. Negative zero is reported as "0" because -0 is not a value
// any source can supply.
func CanonicalNumber(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// Round2 rounds to two decimals: the only rounding the report performs, and
// only on a percentage change.
func Round2(v float64) float64 {
	if !isFinite(v) {
		return v
	}
	return math.Round(v*100) / 100
}

// NumberIndex maps every acceptable number to the fact ids that justify it: the
// exact source value, the exact change, and a percentage change rounded to two
// decimals. A narrative number outside this index is unsupported.
func NumberIndex(facts []Fact) map[string][]string {
	index := make(map[string][]string, len(facts)*4)
	add := func(number, factID string) {
		for _, known := range index[number] {
			if known == factID {
				return
			}
		}
		index[number] = append(index[number], factID)
	}

	for _, f := range facts {
		for _, v := range []*float64{f.Value, f.PreviousValue, f.Change} {
			if v != nil && isFinite(*v) {
				add(CanonicalNumber(*v), f.ID)
			}
		}
		if f.ChangePercent != nil && isFinite(*f.ChangePercent) {
			add(CanonicalNumber(*f.ChangePercent), f.ID)
			add(CanonicalNumber(Round2(*f.ChangePercent)), f.ID)
		}
	}
	return index
}

// metricOrder lists the metric ids in the order they first appear.
func metricOrder(in Input) []string {
	var order []string
	seen := make(map[string]struct{}, len(in.Metrics))
	for _, row := range in.Metrics {
		id := strings.TrimSpace(row.ID)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		order = append(order, id)
	}
	return order
}

// rowsOf returns the supplied rows of one metric id, trimmed, in input order.
func rowsOf(in Input, metricID string) []Row {
	var rows []Row
	for _, row := range in.Metrics {
		if strings.TrimSpace(row.ID) != metricID {
			continue
		}
		rows = append(rows, Row{
			ID:     metricID,
			Period: strings.TrimSpace(row.Period),
			Value:  row.Value,
			Unit:   strings.TrimSpace(row.Unit),
			Source: strings.TrimSpace(row.Source),
		})
	}
	return rows
}

func restatementNote(period string, earlier, last *float64) string {
	if earlier == nil {
		return fmt.Sprintf("restated: %s was supplied without a value and restated as %s", period, CanonicalNumber(*last))
	}
	if last == nil {
		return fmt.Sprintf("restated: %s was supplied as %s and restated without a value", period, CanonicalNumber(*earlier))
	}
	return fmt.Sprintf("restated: %s was supplied as %s and restated as %s", period, CanonicalNumber(*earlier), CanonicalNumber(*last))
}

// percentChange returns the percentage change from base to value, and whether a
// finite percentage exists at all.
func percentChange(base, value float64) (float64, bool) {
	if base == 0 {
		return 0, false
	}
	percent := (value - base) / base * 100
	if !isFinite(percent) {
		return 0, false
	}
	return percent, true
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
