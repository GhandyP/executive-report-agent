# executive-report-agent — v1 evidence-preserving KPI report

Public Go project, built from scratch: one Google ADK (Go v2) agent turns a supplied JSON record of
**multi-period** KPI observations into an executive report whose numbers always come from a
deterministic computation, never from the model. Live dashboards, metrics stores and distribution stay out of v1.

Repo: `github.com/GhandyP/executive-report-agent` (public, MIT). Branch: `feature/executive-report-agent`.
Delivery: `chain_strategy=feature-branch-chain` — work-unit commits on the feature branch, one PR to `main` at slice close (push and PR require the user's explicit OK).
Test runner: `go test ./... -count=1` (plus `gofmt -l .`, `go vet ./...`, `go build ./...`).

## Specs

S1 — user (verbatim): "explora este proyecto , crea un repo publico que la idea es hacerlo desde el inicio pero para que sea un repo dentro de mi portafolio en github , y luego continuar con crear en proyecto con odd, la idea es crear desde el inicio un nuevo programa con go google adk"

S2 — user decisions (verbatim labels): repo name "executive-report-agent"; input scope "Historial de varios períodos"; delivery "Rama feature + PR (recomendado)"; license "MIT (igual que tus otros repos)".

S3 — README "Goal" (verbatim): "Turn supplied KPI or metric text into an evidence-preserving executive report without changing numbers or pretending to read a live dashboard."

S4 — README "Missing elements" → Contract (verbatim): "Define `KPIReportInput{period, metrics[{id,value,unit,source}]}` -> `ExecutiveReport{highlights[], trends[], risks[], questions[], evidence[]}`."

S5 — user decision (verbatim label and description): "Historial de varios períodos" — "Incluye varias observaciones por métrica y calcula variaciones de forma determinista."

S6 — README "Done when" (verbatim): "Every reported number matches a source value or a tested calculation."

S7 — README "Done when" (verbatim): "Missing data and zero denominators are visible, not silently filled."

S8 — README "Done when" (verbatim): "KPI fixtures and tests cover periods, units, and restatements."

S9 — README "Done when" (verbatim): "Distribution requires an identified human approver."

S10 — README "What exists now" (verbatim): "This is intentionally text-only: it does not connect to a metrics store, dashboard, spreadsheet, or report publisher."

S11 — README sample prompt (verbatim): "Write an executive report from these quarterly KPI values without changing the numbers."

S12 — README "Missing elements" → Fixtures and validation (verbatim): "Include missing values, zero denominators, negative changes, mixed units, and restatements; test numerical fidelity and rounding."

**Assumptions** (marked, not user requirements): module path `github.com/GhandyP/executive-report-agent`; input is read as JSON from a file or stdin; model `gemini-flash-latest` with `ADK_MODEL` override and `GOOGLE_API_KEY` credentials, as the case README documents; the whole report is `status: "draft"` because S9 requires a named human approver before distribution.

### Acceptance criteria (v1)

- A1 (S4, S5): the input contract accepts one reporting period label plus metrics carrying several period observations, each with `id`, `value`, `unit`, `source`.
- A2 (S6): every number printed in the report comes from a source observation or a deterministic calculation over them; a narrative number absent from the computed fact index rejects the model answer.
- A3 (S7): a period with no observation is reported as missing; a zero base reports no percentage change instead of a substituted value.
- A4 (S8): fixtures cover missing values, zero denominators, negative changes, mixed units, and restatements; tests pin each.
- A5 (S9): the report is `draft` with an empty `approvedBy`, and the CLI never publishes or distributes.
- A6 (S10): the program reads only supplied input; there is no metrics-store, dashboard, spreadsheet or publisher adapter, and no tool that writes anywhere.
- A7 (S1, S2): the code is a standalone Go module in this repository, with LICENSE, CI, README and ODD documentation.

**Non-goals (v1)**: metrics-store or spreadsheet adapter; live dashboard or API ingestion; scheduled or distributed delivery; PDF/DOCX rendering; multi-agent graphs; semantic attribution of a narrated percentage to a specific fact (the validator checks number membership only, and the README states this limit); approval workflow implementation beyond the draft marker and the approver field.

## Tasks

| ID | Task | Specs | Route | Status / evidence |
| --- | --- | --- | --- | --- |
| T1 | Bootstrap: isolated repo, MIT LICENSE, .gitignore, `go.mod` + ADK Go v2 dependency, CI workflow | S1, S2, A7 | inline | done — `21c7a89` |
| T2 | Input contract + validation with RED tests (`internal/kpi`) | A1, A3 | inline | done — RED `no non-test Go files in internal/kpi` (exit 1), GREEN 16 tests, `1b2255b` |
| T3 | Deterministic facts: deltas, percentage change, zero base, missing period, restatement, mixed units (`internal/kpi/facts.go`) | A3, A4 | inline | done — RED undefined `Fact`/`ComputeFacts`/`CanonicalNumber`/`NumberIndex`, GREEN 27 tests in the package, `6e07198` |
| T4 | ADK agent + prompt + number-membership fidelity validation (`internal/report`) | A2, A6, S11, S12 | inline | done — RED `no non-test Go files in internal/report`, first GREEN blocked by missing go.sum entries (needed `go mod tidy` for the ADK transitive deps), GREEN 16 tests, `da6a4e2` |
| T5 | CLI (`main.go`), fixtures (`testdata/`), README rewrite | A5, A7, S3, S10 | inline | done — RED `undefined: run`, `undefined: exitOK`; GREEN 10 tests; functional: `-dry-run` exit 0 with 8 facts, live run without a key exit 1 and no file written; README rewritten |
| T4 | ADK agent + prompt + number-membership fidelity validation (`internal/report`) | A2, A6, S11, S12 | inline | pending |
| T5 | CLI (`main.go`), fixtures (`testdata/`), README rewrite | A5, A7, S3, S10 | inline | pending |
| T6 | Independent verification of the candidate | A1-A7 | verifier (assess decides tier) | pending |
| T7 | Delivery: push the feature branch and open the PR to `main` | S2 | after explicit user OK | pending |

## Log

L1 — user (verbatim): "explora este proyecto , crea un repo publico que la idea es hacerlo desde el inicio pero para que sea un repo dentro de mi portafolio en github , y luego continuar con crear en proyecto con odd, la idea es crear desde el inicio un nuevo programa con go google adk"

L2 — user decisions (verbatim labels): repo "executive-report-agent"; scope "Historial de varios períodos"; delivery "Rama feature + PR (recomendado)"; license "MIT (igual que tus otros repos)".

L3 — exploration evidence (2026-10-13): this directory had no local `.git`; `git rev-parse --show-toplevel` resolved to `/home/ghandy` (no commits, no remotes), so every Git command before T1 ran from the wrong boundary. T1 fixes it with an isolated `git init` in this directory; `/home/ghandy/.git` is left untouched. `gh` is authenticated as `GhandyP` (`repo`, `workflow`); `gh repo view` confirms `GhandyP/executive-report-agent` exists, `PUBLIC`, description set, no commits yet.

L4 — research evidence (2026-10-13): official Go quickstart requires Go 1.25+ and `google.golang.org/adk/v2`; `proxy.golang.org` reports the newest release `v2.6.0`. Local toolchain is `go1.26.4`, module cache is `/home/ghandy/go/pkg/mod` and holds only `adk@v1.x`, so T1 downloads `v2.6.0` into the local module cache. `llmagent.Config` carries `Model`, `Instruction` and `OutputSchema *genai.Schema`; the parsed structured answer is read from `session.Event.Output`.

L5 — convention evidence (2026-10-13): the portfolio repos `adk-meeting-summary` and `source-grounded-faq-generation` establish the house pattern followed here — MIT LICENSE, `.gitignore`, `go.mod` with ADK v2, `internal/` packages with pure functions plus `testdata/`, `odd/tasks/<feature>.md`, and a CI workflow running `gofmt -l`, `go vet`, `go build`, `go test`. `adk-meeting-summary` redacts its prompt/parser/validation into pure functions because only the model call needs credentials; this feature keeps that boundary.

L6 — decision: the semantics S4 leaves open are fixed here as follows, all deterministic and testable. (1) A repeated period for one metric id is a restatement: the last observation wins and the earlier value stays visible on the fact. (2) A metric id whose unit changes between observations is a contract violation, not a silent conversion. (3) A percentage change is computed against the previous available observation of the same metric; with a zero or missing base the fact carries no percentage and a visible flag instead. (4) The fact number index accepts a source value or delta exactly, and a percentage change rounded to two decimals; anything else in the narrative rejects the answer. (5) `-dry-run` prints the deterministic facts and the model payload without credentials.

L7 — decision: the input model is the flat row `metrics[{id, period, value, unit, source}]`, which is S4's documented row plus the period it belongs to. Keeping the unit on each row is what makes the two S12 cases real: a metric reported in two units is a validation error, and several periods for one id are ordinary multi-period history. `value: null` is valid and means the period is known with no value.

L8 — ENVIRONMENT INCIDENT (2026-10-13): `/tmp` is a 3.9 GB tmpfs at 100% (≈3.0 GB `/tmp/go-port-verify.oRpSy3`, 763 MB `/tmp/odd-verify.S2mSSy`, plus stale `/tmp/go-build*`), so the first `go test` failed with `write /tmp/go-build135139549/b001/_testmain.go: no space left on device`. Nothing outside this repository was deleted. Workaround in force for every command on this branch: `TMPDIR="$PWD/.tmp"` (git-ignored). Freeing those orphan directories is the user's call and is reported at close.

L9 — evidence (T1): `21c7a89` on `feature/executive-report-agent`. `go mod init` plus `go get google.golang.org/adk/v2@v2.6.0` and `genai@v1.74.0` resolved from `proxy.golang.org`; `go.mod` carries `go 1.26.6` and the local toolchain directive switched to the cached go1.26.6. `gofmt -l .` prints nothing; `go build ./...`, `go vet ./...` and `go test ./... -count=1` all exit 0.

L10 — evidence (T2): RED observed as `github.com/GhandyP/executive-report-agent/internal/kpi: no non-test Go files` with exit 1. First GREEN run exposed a wrong expectation in my own test, not in the rule: two input-level subtests asserted that the message names `metric[0]`, which only row-level errors do. The test was corrected (input errors name the input, row errors name the row) without weakening any rule, and `go test ./internal/kpi/ -count=1` then passed with 16 tests. Commit id is recorded in the T3 entry.

L11 — evidence (T3): commit `1b2255b` holds T2. T3 RED observed as `undefined: kpi.Fact`, `undefined: kpi.ComputeFacts`, `undefined: kpi.CanonicalNumber`, `undefined: kpi.NumberIndex`; GREEN as `ok github.com/GhandyP/executive-report-agent/internal/kpi` with 27 tests in the package. Redesign recorded while implementing: the fact set of a metric is *its own* periods plus the reporting period, not the union of every period in the document. The first implementation consumed a document-wide period universe, which would have produced a spurious "missing" fact for every metric in every period some other metric happened to supply; the implementation and its two tests now agree that only the reporting period is forced, and only the metric's own history is walked for the base of a change. `gofmt -l .` prints nothing and `go vet ./...` exits 0.

L12 — evidence (T4, T5): T4 commit `da6a4e2` (16 tests in `internal/report`), T5 commit recorded below (10 tests in the root package; 53 across the three packages). RED for T4 was `no non-test Go files in internal/report`; the first GREEN was blocked by `missing go.sum entry` for `github.com/google/uuid`, `rsc.io/omap`, `rsc.io/ordered`, `github.com/google/jsonschema-go/jsonschema` and `github.com/google/safehtml/template`, which `go mod tidy` resolved (first compile of the ADK graph took 3m27s while sibling Pi sessions ran their own builds). RED for T5 was `undefined: run`, `undefined: exitOK`, `undefined: exitFailure`, `undefined: exitUsage`.

L13 — two defects were in my tests, not in the code, and both were fixed without weakening a rule. (1) `TestFactsPreserveSuppliedDigits` matched `"value":124` as a string, which a pretty-printed JSON document never contains (it writes `"value": 124`); the test now unmarshals the facts and asserts the exact number plus the restatement note, so it checks the value instead of the whitespace. (2) The temporary `TMPDIR="$PWD/.tmp"` workaround from L8 put Go's build scratch inside the repository, which made `gofmt -l .` report generated `go-build*` files and would have broken the CI formatting gate if it were ever run that way. `/tmp` had returned to 20M/1% used by the time this was found, the directory was removed, and every check now runs with the default `TMPDIR`; `/.tmp/` stays in `.gitignore` in case the workaround is needed again.

L14 — functional evidence (T5), observed on the built binary with no `GOOGLE_API_KEY` in the environment: `go run . -in testdata/kpis_ok.json -dry-run` exits 0, prints 8 facts (three revenue periods, two gross-margin periods, two order periods, one missing churn period) and the exact model payload on stderr; `go run . -in testdata/kpis_ok.json -out /tmp/er-live.json` exits 1 with `agent: GOOGLE_API_KEY is required to call the model` and creates no output file. **Not verified**: the live Gemini path. No API key exists in this environment, so no model call was ever executed and nothing here claims a live result; the README states that the tests never call a model.
