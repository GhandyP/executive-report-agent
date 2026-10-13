// Package kpi holds the input contract for one executive KPI report and the
// deterministic facts computed from the supplied rows.
//
// Everything in this package is a pure function of its arguments: it never
// calls a model, a network service or the clock, so each rule below is testable
// without credentials.
package kpi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Row is one supplied KPI value for one period: the documented contract's
// metrics[{id,value,unit,source}] plus the period the value belongs to, so one
// metric can be supplied for several periods.
//
// A nil Value means the period is known but its value is missing. It is valid
// input and the report shows the gap instead of filling it.
type Row struct {
	ID     string   `json:"id"`
	Period string   `json:"period"`
	Value  *float64 `json:"value"`
	Unit   string   `json:"unit"`
	Source string   `json:"source"`
}

// Input is one reporting request: the reporting period label plus every
// supplied row. Periods other than the reporting one are the comparison
// history the deterministic calculations need.
type Input struct {
	Period  string `json:"period"`
	Metrics []Row  `json:"metrics"`
}

// Load decodes the input document from path, or from stdin when path is "-".
//
// It only decodes: unknown fields, malformed JSON and unreadable files are
// errors, while the contract rules belong to Validate so the caller can choose
// how to report an incomplete document.
func Load(path string, stdin io.Reader) (Input, error) {
	reader := stdin
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return Input{}, fmt.Errorf("open %s: %w", path, err)
		}
		defer file.Close()
		reader = file
	}
	if reader == nil {
		return Input{}, errors.New(`reading "-" requires a stdin reader`)
	}

	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()

	var in Input
	if err := decoder.Decode(&in); err != nil {
		return Input{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return in, nil
}

// Normalize returns a copy with every text field trimmed. Values are copied
// unchanged: trimming must never alter a supplied number.
func Normalize(in Input) Input {
	out := Input{Period: strings.TrimSpace(in.Period)}
	if in.Metrics != nil {
		out.Metrics = make([]Row, 0, len(in.Metrics))
		for _, row := range in.Metrics {
			out.Metrics = append(out.Metrics, Row{
				ID:     strings.TrimSpace(row.ID),
				Period: strings.TrimSpace(row.Period),
				Value:  row.Value,
				Unit:   strings.TrimSpace(row.Unit),
				Source: strings.TrimSpace(row.Source),
			})
		}
	}
	return out
}
