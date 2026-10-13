// Package report turns computed KPI facts into an executive report: the facts
// themselves plus the narrative one Google ADK agent writes about them.
//
// The model boundary is deliberately narrow. Everything that can be checked
// without credentials - the prompt, the output schema, the parser and the
// number-fidelity validation - is a pure function, and the facts a report
// carries are always the ones computed before the model was called, never
// anything the model returned.
package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/GhandyP/executive-report-agent/internal/kpi"
)

// StatusDraft is the only status a report can have: distribution needs an
// identified human approver, which this program cannot supply.
const StatusDraft = "draft"

// Narrative violations reported by ParseNarrative. Each is a distinct error
// value so the caller can tell an unusable answer from a missing one.
var (
	ErrNoNarrative       = errors.New("the model returned no narrative items")
	ErrEmptyItem         = errors.New("narrative item text is empty")
	ErrMissingEvidence   = errors.New("narrative item cites no fact")
	ErrUnknownCitation   = errors.New("narrative item cites an unknown fact")
	ErrUnsupportedNumber = errors.New("narrative contains a number that no source value or calculation supports")
)

// Item is one narrative statement with the facts that support it.
type Item struct {
	Text        string   `json:"text"`
	EvidenceIDs []string `json:"evidenceIDs"`
}

// Narrative is the model's whole answer, in the documented sections.
type Narrative struct {
	Highlights []Item `json:"highlights"`
	Trends     []Item `json:"trends"`
	Risks      []Item `json:"risks"`
	Questions  []Item `json:"questions"`
}

// Report is the deliverable: the deterministic facts, the narrative about them,
// and the draft marker that keeps distribution a human decision.
type Report struct {
	Period     string     `json:"period"`
	Status     string     `json:"status"`
	ApprovedBy string     `json:"approvedBy"`
	Facts      []kpi.Fact `json:"facts"`
	Narrative  Narrative  `json:"narrative"`
}

// sections pairs each narrative section with the name used in error messages.
func (n Narrative) sections() []struct {
	name  string
	items []Item
} {
	return []struct {
		name  string
		items []Item
	}{
		{"highlights", n.Highlights},
		{"trends", n.Trends},
		{"risks", n.Risks},
		{"questions", n.Questions},
	}
}

// ParseNarrative decodes the model's answer for these facts and validates it.
//
// An answer is rejected, never repaired, when it is malformed, empty, cites a
// fact that does not exist, states an item with no evidence, or contains a
// number that no supplied value and no tested calculation produced. The facts
// on the returned report are the ones passed in: the model contributes prose,
// never numbers.
func ParseNarrative(in kpi.Input, facts []kpi.Fact, raw []byte) (Report, error) {
	var narrative Narrative
	if err := json.Unmarshal(raw, &narrative); err != nil {
		return Report{}, fmt.Errorf("decode model answer: %w", err)
	}

	known := make(map[string]struct{}, len(facts))
	for _, f := range facts {
		known[f.ID] = struct{}{}
	}
	numbers := kpi.NumberIndex(facts)
	mask := knownTokens(in, facts)

	items := 0
	for _, section := range narrative.sections() {
		for i, item := range section.items {
			items++
			where := fmt.Sprintf("%s[%d]", section.name, i)

			if strings.TrimSpace(item.Text) == "" {
				return Report{}, fmt.Errorf("%s: %w", where, ErrEmptyItem)
			}
			if len(item.EvidenceIDs) == 0 {
				return Report{}, fmt.Errorf("%s: %w", where, ErrMissingEvidence)
			}
			for _, id := range item.EvidenceIDs {
				if _, ok := known[strings.TrimSpace(id)]; !ok {
					return Report{}, fmt.Errorf("%s: %w: %q", where, ErrUnknownCitation, id)
				}
			}
			for _, number := range numbersIn(item.Text, mask) {
				if _, ok := numbers[number]; !ok {
					return Report{}, fmt.Errorf("%s: %w: %s (accepted numbers: %s)",
						where, ErrUnsupportedNumber, number, acceptedList(numbers))
				}
			}
		}
	}
	if items == 0 {
		return Report{}, ErrNoNarrative
	}

	return Report{
		Period:     in.Period,
		Status:     StatusDraft,
		ApprovedBy: "",
		Facts:      facts,
		Narrative:  narrative,
	}, nil
}

// numberPattern matches a signed decimal number, with either separator used for
// grouping or for decimals.
var numberPattern = regexp.MustCompile(`[+-]?\d+(?:[.,]\d+)*`)

// numbersIn returns the numbers of text that are not part of an allowed token,
// normalized the way the number index stores them.
func numbersIn(text string, mask []string) []string {
	for _, token := range mask {
		text = strings.ReplaceAll(text, token, " ")
	}

	var found []string
	for _, match := range numberPattern.FindAllString(text, -1) {
		found = append(found, normalizeNumber(match))
	}
	return found
}

// normalizeNumber drops grouping separators and a leading plus sign, so that
// "1,234.5" and "+1234.5" are compared as the source value they denote.
func normalizeNumber(raw string) string {
	return strings.TrimPrefix(strings.ReplaceAll(raw, ",", ""), "+")
}

// knownTokens lists every piece of text that may contain digits without being a
// source value: fact ids, metric ids, unit names, and the fragments of every
// period label (so "2026-Q3", "Q3" and "2026" are labels, not numbers).
func knownTokens(in kpi.Input, facts []kpi.Fact) []string {
	var tokens []string
	seen := map[string]struct{}{}
	add := func(token string) {
		token = strings.TrimSpace(token)
		if token == "" {
			return
		}
		if _, ok := seen[token]; ok {
			return
		}
		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}

	for _, f := range facts {
		add(f.ID)
		add(f.MetricID)
		add(f.Unit)
		for _, period := range []string{f.Period, f.PreviousPeriod} {
			add(period)
			for _, fragment := range strings.FieldsFunc(period, func(r rune) bool {
				return r == '-' || r == '/' || r == '_' || r == '.'
			}) {
				if strings.ContainsAny(fragment, "0123456789") {
					add(fragment)
				}
			}
		}
	}
	add(in.Period)

	// Longest first, so a fact id is masked before the metric id inside it.
	sort.Slice(tokens, func(i, j int) bool { return len(tokens[i]) > len(tokens[j]) })
	return tokens
}

// acceptedList renders the accepted numbers for an error message, bounded so a
// large fact set cannot produce an unreadable error.
func acceptedList(numbers map[string][]string) string {
	list := make([]string, 0, len(numbers))
	for number := range numbers {
		list = append(list, number)
	}
	sort.Strings(list)
	const limit = 12
	if len(list) > limit {
		rest := len(list) - limit
		list = append(list[:limit], fmt.Sprintf("and %d more", rest))
	}
	return strings.Join(list, ", ")
}
