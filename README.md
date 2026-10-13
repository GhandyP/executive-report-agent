# executive-report-agent

Turn supplied multi-period KPI observations into an executive report whose **every number comes
from a deterministic computation** over the values you supplied — and whose narrative prose is
written by one Google ADK (Go v2) agent that is never allowed to introduce a number of its own.

The program reads only the input it is given. It does not connect to a metrics store, a dashboard,
a spreadsheet or a publisher, and it never distributes anything: every report it prints is marked
`draft`, because distribution requires an identified human approver.

```
                 ┌──────────────────────────┐
 kpis.json  ───▶ │ internal/kpi             │  validation + facts
                 │  contract, validation,   │  (values, changes, percentages,
                 │  deterministic facts     │   gaps, restatements)
                 └───────────┬──────────────┘
                             │ prompt (facts only)
                 ┌───────────▼──────────────┐
                 │ internal/report          │  ADK agent → narrative
                 │  prompt, schema, agent,  │  prose only, cites fact ids
                 │  number-fidelity check   │
                 └───────────┬──────────────┘
                             │
                   report.json (facts + narrative, status: draft)
```

## Requirements

- Go 1.26 or later (the module pins `go 1.26.6`, which ADK Go v2 requires).
- A Gemini API key in `GOOGLE_API_KEY` **only** for a live run. `-dry-run` needs no credentials.

## Quick start

Validate the input and inspect exactly what the model would receive, with no key and no model call:

```bash
go run . -in testdata/kpis_ok.json -dry-run
```

`-dry-run` prints the deterministic facts as JSON on standard output and the exact model payload on
standard error, so the prompt is reproducible without spending a token.

Produce a report:

```bash
export GOOGLE_API_KEY="your-key"
go run . -in testdata/kpis_ok.json -out report.json
```

Override the model with `-model gemini-2.5-pro` or `ADK_MODEL`; the default is
`gemini-flash-latest`.

## Input contract

```json
{
  "period": "2026-Q3",
  "metrics": [
    {"id": "revenue", "period": "2026-Q2", "value": 1050000, "unit": "USD", "source": "finance/2026-monthly.csv"},
    {"id": "revenue", "period": "2026-Q3", "value": 1120000, "unit": "USD", "source": "finance/2026-monthly.csv"},
    {"id": "churn",   "period": "2026-Q3", "value": null,    "unit": "count", "source": "ops/support.csv"}
  ]
}
```

- `period` is the reporting period. Every other supplied period is comparison history.
- `value` may be `null`: the period is known and its value is missing.
- A repeated `(id, period)` is a **restatement**: the last row wins and the earlier value stays
  visible on the fact.
- One metric id must keep one unit. A change of unit is refused, never converted.
- Unknown fields are rejected, so a typo cannot silently drop data.
- `src`-level privacy is yours: `.gitignore` excludes `testdata/private/`, `*.xlsx`, `*.csv` and
  `/reports/`.

## What the facts add

For each metric and each period, the program computes — before the model is called:

| Situation | Reported as |
| --- | --- |
| A value | `value`, plus `change` and `changePercent` against the previous period that had a value |
| The previous value is `0` | `change` with no percentage, and `zeroBase: true` |
| `value: null` | `missing: true` with no value, no change and no percentage |
| The reporting period was never supplied | a fact marked `missing`, so the gap is visible instead of absent |
| A period supplied twice | the last value, plus `restated: true` and a note naming the earlier value |
| A percentage that is not a finite number | no percentage, plus a note explaining it |
| One metric id in two units | a validation error that names both units; nothing is printed |

Every calculation lives in `internal/kpi` and is covered by deterministic tests that need no
credentials.

## The number-fidelity rule

The model writes prose only. A narrative item is accepted only when it:

1. cites at least one fact id (`<metricId>@<period>`) that exists;
2. contains no number outside the **number index** — a supplied value, a supplied previous value, a
   computed change, or a computed percentage rounded to two decimals.

Period labels (`2026-Q3`) and their fragments (`2026`, `Q3`) are labels, not numbers, and are
allowed. An answer that states any other number is **rejected, never repaired**: the CLI exits `1`,
names the number and lists the accepted ones. The facts a report carries are always the ones
computed before the call, so the model cannot change a figure even if it wanted to.

## Report shape

```json
{
  "period": "2026-Q3",
  "status": "draft",
  "approvedBy": "",
  "facts": [ {"id": "revenue@2026-Q3", "value": 1120000, "change": 70000, "changePercent": 6.67, "...": "..."} ],
  "narrative": {
    "highlights": [ {"text": "Revenue reached 1120000.", "evidenceIDs": ["revenue@2026-Q3"]} ],
    "trends": [], "risks": [], "questions": []
  }
}
```

## Command line

| Flag | Meaning |
| --- | --- |
| `-in` | KPI input JSON file, or `-` for stdin (default `-`) |
| `-out` | Report JSON file, or `-` for stdout (default `-`); a file is written `0600` |
| `-model` | Gemini model name |
| `-dry-run` | Validate, print the facts and the model payload, call no model |
| `-pretty` | Indent the JSON output (default `true`) |

| Environment | Meaning |
| --- | --- |
| `GOOGLE_API_KEY` | Required for a live run; `-dry-run` does not need it |
| `ADK_MODEL` | Overrides the default model name |

| Exit code | Meaning |
| --- | --- |
| `0` | The report was produced (or help was printed) |
| `1` | The input, the model or the narrative failed validation |
| `2` | The command line was wrong |

Gaps are announced on standard error before the report, for example
`warning: churn has no value for 2026-Q3`, and a live run always ends with
`draft: distribution requires an identified human approver`.

## Tests

```bash
gofmt -l .
go vet ./...
go test ./... -count=1
```

The suite never calls a model: the prompt, the output schema, the parser and the fidelity rules are
pure functions, so everything except the network call itself is covered offline. CI
(`.github/workflows/ci.yml`) runs the same four commands.

## Out of scope (deliberate)

- No metrics-store, dashboard, spreadsheet or API ingestion: you supply the values.
- No PDF/DOCX rendering, no email, no publishing, no scheduling.
- No approval workflow: the report is stamped `draft` and the program stops there.
- The fidelity check is about *number membership*. It cannot prove that a narrated percentage is
  attributed to the metric the sentence talks about; the evidence ids and the `facts` array are what
  a reader checks that against.

## Layout

```
main.go                    CLI: input, dry run, agent call, exit codes
internal/kpi/              contract, validation, deterministic facts, number index
internal/report/           prompt renderer, narrative validation, ADK agent
testdata/                  fixtures: normal, zero base + gap, restated, mixed units
odd/tasks/executive-report-agent-v1.md   the specification of record for this branch
```

Built with Organic Driven Development: the feature document above holds the specs (`S1`-`S12`), the
acceptance criteria (`A1`-`A7`), the tasks and the evidence log.

## License

MIT — see [LICENSE](LICENSE).
