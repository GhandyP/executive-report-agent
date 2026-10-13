package report

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/GhandyP/executive-report-agent/internal/kpi"
)

const (
	// DefaultModel is the Gemini model this program documents.
	DefaultModel = "gemini-flash-latest"
	// ModelEnv overrides DefaultModel.
	ModelEnv = "ADK_MODEL"
	// APIKeyEnv is the environment variable the model client reads.
	APIKeyEnv = "GOOGLE_API_KEY"
	// AppName identifies the ADK application that owns the sessions.
	AppName   = "executive-report"
	agentName = "executive_report_writer"
	userID    = "executive-report-cli"
)

// ErrMissingAPIKey reports a model call attempted without credentials.
var ErrMissingAPIKey = fmt.Errorf("%s is required to call the model", APIKeyEnv)

// ErrNoAgentOutput reports an agent turn that produced no narrative at all.
var ErrNoAgentOutput = errors.New("the agent finished without a narrative")

// instruction is the whole contract handed to the model. The program never asks
// the model for numbers: the facts already carry them.
const instruction = `You write the narrative of ONE executive report from a set of already computed facts.

Rules:
- Only use the supplied facts. Never call a tool, and never read a dashboard,
  a metrics store, a spreadsheet or the network on your own.
- Never invent data and never compute, round, re-derive or estimate a number yourself:
  your text may contain a supplied value, a supplied change, or a supplied percentage,
  written exactly as the facts state them, and no other digits at all.
  When you need a count, write it in words.
- Period labels such as 2026-Q3 are labels, not numbers, and may be written as they appear.
- Every item needs at least one evidenceIDs entry, copied exactly from the id of a fact
  that supports it. An item without evidence is rejected.
- highlights: what the facts show for the reporting period. trends: how the facts moved
  against the previous period. risks: what the facts cannot support. questions: what a
  human must confirm before the report is used.
- A fact whose value is missing has no value: say it is missing, never assume one.
  A fact with no percentage change has none: never attach a percentage to it.
- This report is a draft for internal review and is not for distribution: never claim it
  was approved, published or sent, and never write it as a final decision.
- Write short, plain sentences: no headings, no tables, no markdown.
`

// Instruction returns the agent instruction. Tests pin the fidelity rules and
// the text-only boundary on it.
func Instruction() string { return instruction }

// OutputSchema describes the narrative the model must return. The facts and the
// draft marker are deliberately absent: the program supplies them, so the model
// cannot get them wrong.
func OutputSchema() *genai.Schema {
	section := func(description string) *genai.Schema {
		return &genai.Schema{
			Type:        genai.TypeArray,
			Description: description,
			Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"text": {Type: genai.TypeString, Description: "One or two short sentences."},
					"evidenceIDs": {
						Type:        genai.TypeArray,
						Description: "Ids of the facts that support this item, copied exactly.",
						Items:       &genai.Schema{Type: genai.TypeString},
					},
				},
				Required: []string{"text", "evidenceIDs"},
			},
		}
	}

	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"highlights": section("What the facts show for the reporting period."),
			"trends":     section("How the facts moved against the previous period."),
			"risks":      section("What the facts cannot support, including gaps."),
			"questions":  section("What a human must confirm before the report is used."),
		},
		Required: []string{"highlights", "trends", "risks", "questions"},
	}
}

// Config selects the model and the credentials. Both default to the environment.
type Config struct {
	// ModelName defaults to ADK_MODEL and then to DefaultModel.
	ModelName string
	// APIKey defaults to APIKeyEnv.
	APIKey string
}

// Agent runs one ADK agent over the facts of one report.
type Agent struct {
	runner *runner.Runner
	runs   atomic.Uint64
}

// NewAgent builds the single-agent narrative writer. It fails before any
// network call when no credentials are available, and it binds no tools, so the
// agent cannot read or write anything beyond the facts it is given.
func NewAgent(ctx context.Context, cfg Config) (*Agent, error) {
	modelName := firstNonEmpty(cfg.ModelName, os.Getenv(ModelEnv), DefaultModel)
	apiKey := firstNonEmpty(cfg.APIKey, os.Getenv(APIKeyEnv))
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	model, err := gemini.NewModel(ctx, modelName, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("create model %q: %w", modelName, err)
	}

	writer, err := llmagent.New(llmagent.Config{
		Name:         agentName,
		Model:        model,
		Description:  "Writes the narrative of one executive KPI report from facts that were computed before the call.",
		Instruction:  Instruction(),
		OutputSchema: OutputSchema(),
	})
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}

	run, err := runner.NewInMemory(AppName, writer)
	if err != nil {
		return nil, fmt.Errorf("create runner: %w", err)
	}

	return &Agent{runner: run}, nil
}

// Draft computes the facts of one input, asks the model for a narrative about
// them, and validates the answer. The report it returns always carries the
// computed facts, whatever the model wrote.
func (a *Agent) Draft(ctx context.Context, in kpi.Input) (Report, error) {
	if err := kpi.Validate(in); err != nil {
		return Report{}, err
	}
	normalized := kpi.Normalize(in)
	facts := kpi.ComputeFacts(normalized)

	// A fresh session per run keeps one report's answer out of the next prompt.
	sessionID := fmt.Sprintf("%s-%d", normalized.Period, a.runs.Add(1))
	prompt := genai.NewContentFromText(RenderPrompt(normalized, facts), genai.RoleUser)

	var raw, text string
	for event, err := range a.runner.Run(ctx, userID, sessionID, prompt, agent.RunConfig{}) {
		if err != nil {
			return Report{}, fmt.Errorf("run agent: %w", err)
		}
		if event == nil {
			continue
		}
		// ADK stores the value parsed against OutputSchema on the terminal event.
		// Read it wherever it appears and keep the last one, so a differently
		// marked final event cannot make the narrative disappear.
		if event.Output != nil {
			encoded, err := json.Marshal(event.Output)
			if err != nil {
				return Report{}, fmt.Errorf("encode agent output: %w", err)
			}
			raw = string(encoded)
		}
		if event.IsFinalResponse() {
			if answer := eventText(event); answer != "" {
				text = answer
			}
		}
	}

	if raw == "" {
		if text == "" {
			return Report{}, ErrNoAgentOutput
		}
		raw = text
	}
	return ParseNarrative(normalized, facts, []byte(raw))
}

func eventText(event *session.Event) string {
	if event == nil || event.Content == nil {
		return ""
	}
	var b strings.Builder
	for _, part := range event.Content.Parts {
		if part != nil {
			b.WriteString(part.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
