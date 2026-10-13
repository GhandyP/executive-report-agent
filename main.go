// Command executive-report-agent turns a supplied JSON record of multi-period
// KPI observations into an executive report whose every number comes from a
// deterministic computation over the supplied values.
//
// It reads only the input it is given. It has no metrics-store, dashboard,
// spreadsheet or publisher adapter, no tool that writes anywhere, and no
// delivery step: the report it prints is always a draft.
//
// Exit codes:
//
//	0  the report was produced (or help was printed)
//	1  the input, the model, or the narrative failed validation
//	2  the command line was wrong
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/GhandyP/executive-report-agent/internal/kpi"
	"github.com/GhandyP/executive-report-agent/internal/report"
)

const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

const usage = `executive-report-agent turns supplied multi-period KPI observations into an executive report.

Usage:
  executive-report-agent [flags]

Flags:
  -in string     KPI input JSON file to read, or "-" for stdin (default "-")
  -out string    file for the report JSON, or "-" for stdout (default "-")
  -model string  Gemini model name (default: ADK_MODEL, then ` + report.DefaultModel + `)
  -dry-run       validate the input, print the facts and the model payload, and call no model
  -pretty        indent the JSON output (default true)

Environment:
  ` + report.APIKeyEnv + `  required to call the model; not needed with -dry-run
  ` + report.ModelEnv + `       overrides the default model name

Every number a report carries is a supplied value or a calculation over them; a
narrative that states any other number is rejected. Missing values and zero bases
are reported as gaps, never filled. The report is always marked draft, because
distribution requires an identified human approver.

Exit codes:
  0  the report was produced (or help was printed)
  1  the input, the model, or the narrative failed validation
  2  the command line was wrong
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("executive-report-agent", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, usage) }

	inPath := flags.String("in", "-", "KPI input JSON file, or \"-\" for stdin")
	outPath := flags.String("out", "-", "file for the report JSON, or \"-\" for stdout")
	modelName := flags.String("model", "", "Gemini model name")
	dryRun := flags.Bool("dry-run", false, "validate the input and print the model payload without calling a model")
	pretty := flags.Bool("pretty", true, "indent the JSON output")

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n\n", flags.Arg(0))
		flags.Usage()
		return exitUsage
	}

	in, err := kpi.Load(*inPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "input: %v\n", err)
		return exitFailure
	}
	if err := kpi.Validate(in); err != nil {
		fmt.Fprintf(stderr, "input: %v\n", err)
		return exitFailure
	}
	in = kpi.Normalize(in)

	facts := kpi.ComputeFacts(in)
	for _, warning := range warningsOf(facts) {
		fmt.Fprintf(stderr, "warning: %s\n", warning)
	}

	if *dryRun {
		fmt.Fprint(stderr, report.RenderPrompt(in, facts))

		encoded, err := marshal(facts, *pretty)
		if err != nil {
			fmt.Fprintf(stderr, "encode facts: %v\n", err)
			return exitFailure
		}
		if err := writeOutput(*outPath, encoded, stdout); err != nil {
			fmt.Fprintf(stderr, "write facts: %v\n", err)
			return exitFailure
		}
		return exitOK
	}

	ctx := context.Background()
	agent, err := report.NewAgent(ctx, report.Config{ModelName: *modelName})
	if err != nil {
		fmt.Fprintf(stderr, "agent: %v\n", err)
		return exitFailure
	}

	result, err := agent.Draft(ctx, in)
	if err != nil {
		fmt.Fprintf(stderr, "report: %v\n", err)
		return exitFailure
	}

	fmt.Fprintf(stderr, "draft: distribution requires an identified human approver\n")

	encoded, err := marshal(result, *pretty)
	if err != nil {
		fmt.Fprintf(stderr, "encode report: %v\n", err)
		return exitFailure
	}
	if err := writeOutput(*outPath, encoded, stdout); err != nil {
		fmt.Fprintf(stderr, "write report: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// warningsOf lists the gaps a human should see before reading the narrative.
func warningsOf(facts []kpi.Fact) []string {
	var warnings []string
	for _, f := range facts {
		switch {
		case f.Missing:
			warnings = append(warnings, fmt.Sprintf("%s has no value for %s", f.MetricID, f.Period))
		case f.ZeroBase:
			warnings = append(warnings, fmt.Sprintf("%s has no percentage change for %s: the previous value is zero", f.MetricID, f.Period))
		case f.Restated:
			warnings = append(warnings, fmt.Sprintf("%s %s: %s", f.MetricID, f.Period, f.Note))
		}
	}
	return warnings
}

func marshal(value any, pretty bool) ([]byte, error) {
	var (
		encoded []byte
		err     error
	)
	if pretty {
		encoded, err = json.MarshalIndent(value, "", "  ")
	} else {
		encoded, err = json.Marshal(value)
	}
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

// writeOutput writes to path, or to stdout when path is "-". Reports can carry
// financial values, so a file is created readable only by its owner.
func writeOutput(path string, data []byte, stdout io.Writer) error {
	if path == "-" {
		_, err := stdout.Write(data)
		return err
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o600)
}
