package report

import (
	"strings"

	"github.com/GhandyP/executive-report-agent/internal/kpi"
)

// RenderPrompt turns the computed facts into the text payload the agent
// receives. It is a pure function: the same input always renders the same
// bytes, so what the model saw can be reprinted later.
//
// The prompt carries every fact id (the only citations the narrative may use)
// and every number the narrative may repeat: the supplied value, the change,
// and the percentage change already rounded for reading.
func RenderPrompt(in kpi.Input, facts []kpi.Fact) string {
	var b strings.Builder
	b.WriteString("reporting period: " + in.Period + "\n")
	b.WriteString("facts (authoritative; copy these numbers exactly):\n")

	for _, f := range facts {
		b.WriteString("- id " + f.ID + " | metric " + f.MetricID + " | unit " + f.Unit)
		switch {
		case f.Value == nil:
			b.WriteString(" | value missing")
		default:
			b.WriteString(" | value " + kpi.CanonicalNumber(*f.Value))
			if f.PreviousPeriod != "" && f.PreviousValue != nil && f.Change != nil {
				b.WriteString(" | previous " + f.PreviousPeriod + " = " + kpi.CanonicalNumber(*f.PreviousValue))
				b.WriteString(" | change " + kpi.CanonicalNumber(*f.Change))
			}
			if f.ChangePercent != nil {
				b.WriteString(" | percentChange " + kpi.CanonicalNumber(kpi.Round2(*f.ChangePercent)))
			}
		}
		if f.Note != "" {
			b.WriteString(" | note " + f.Note)
		}
		b.WriteString(" | source " + f.Source + "\n")
	}
	return b.String()
}
