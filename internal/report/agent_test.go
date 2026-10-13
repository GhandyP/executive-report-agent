package report_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/genai"

	"github.com/GhandyP/executive-report-agent/internal/report"
)

// S6, S11: the instruction is where the fidelity promise is handed to the
// model, so the rules must be present verbatim in it.
func TestInstructionStatesTheFidelityRules(t *testing.T) {
	instruction := report.Instruction()

	for _, want := range []string{
		"Only use the supplied facts",
		"never compute",
		"evidenceIDs",
		"digits",
		"not for distribution",
	} {
		if !strings.Contains(instruction, want) {
			t.Errorf("instruction is missing %q", want)
		}
	}
}

// S4: the model answers with the documented narrative arrays and nothing else.
func TestOutputSchemaDescribesTheNarrative(t *testing.T) {
	schema := report.OutputSchema()

	if schema.Type != genai.TypeObject {
		t.Fatalf("schema type = %v, want object", schema.Type)
	}
	want := []string{"highlights", "trends", "risks", "questions"}
	for _, key := range want {
		property, ok := schema.Properties[key]
		if !ok {
			t.Fatalf("schema is missing %q", key)
		}
		if property.Type != genai.TypeArray {
			t.Errorf("%s type = %v, want array", key, property.Type)
		}
		if property.Items == nil {
			t.Fatalf("%s has no item schema", key)
		}
		if !reflect.DeepEqual(property.Items.Required, []string{"text", "evidenceIDs"}) {
			t.Errorf("%s item required = %v, want text and evidenceIDs", key, property.Items.Required)
		}
	}
	if !reflect.DeepEqual(schema.Required, want) {
		t.Errorf("required = %v, want %v", schema.Required, want)
	}
}

// A6: without credentials the agent fails before any network call.
func TestNewAgentRefusesToRunWithoutCredentials(t *testing.T) {
	t.Setenv(report.APIKeyEnv, "")

	_, err := report.NewAgent(context.Background(), report.Config{})
	if !errors.Is(err, report.ErrMissingAPIKey) {
		t.Fatalf("NewAgent = %v, want ErrMissingAPIKey", err)
	}
	if !strings.Contains(err.Error(), report.APIKeyEnv) {
		t.Errorf("error = %v, want it to name %s", err, report.APIKeyEnv)
	}
}

// S10, A6: the agent has no tool that could read or write anything.
func TestAgentInstructionForbidsLiveDataAndDistribution(t *testing.T) {
	instruction := report.Instruction()

	for _, want := range []string{"dashboard", "metrics store", "publish"} {
		if !strings.Contains(instruction, want) {
			t.Errorf("instruction is missing %q", want)
		}
	}
}
