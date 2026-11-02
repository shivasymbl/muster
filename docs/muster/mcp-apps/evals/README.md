# Muster MCP Apps verification harness

This directory is a planned, implementation-dependent verification contract, not application code or a claim that its suites have run. Red or blocked gates before implementation are expected: missing named tests, missing npm scripts, and absent real-host evidence cannot produce a passing release state.

## Run a gate

Node.js, Bash, Go, and npm are required for the corresponding checks. Set `PROJECT_ROOT` explicitly to an absolute repository checkout path or a path relative to the handoff package root (`Muster-MCP-Apps-Handoff`), not the invocation working directory; the runner expects `server/go.mod` and `mcp-app/package.json` under that repository root.

```bash
# From a directory containing both the project checkout and this handoff:
PROJECT_ROOT=../Muster bash Muster-MCP-Apps-Handoff/evals/T0.sh
PROJECT_ROOT=../Muster bash Muster-MCP-Apps-Handoff/evals/F2.sh
PROJECT_ROOT=/absolute/path/to/Muster bash Muster-MCP-Apps-Handoff/evals/run-all.sh
# Read-only implementation slice; never reports full release readiness:
PROJECT_ROOT=../Muster bash Muster-MCP-Apps-Handoff/evals/run.sh read-only
```

The paths above are examples, not assertions about the current workspace. Do not point this at production or run tests that send actual invitations, write calendars, or message real participants; configure isolated authorized fixtures through the application's existing test setup first.

## Entry points and contracts

The canonical validator-facing layout is `tasks/T0.sh`–`T7.sh` plus `tasks/T1.5.sh`, `acceptance/F1.sh`–`F7.sh`, and `holdout/S1.sh`–`S6.sh`. The manifest exposes `task_verifications`, `acceptance_tests` (A1–A29 mapped to their F wrappers), and `holdout_evals` arrays with paths relative to `evals`; each forwards to the same runner, while the root-level wrappers remain compatible.

| Wrappers | Contract |
|---|---|
| `T0.sh` | Discover and execute the focused baseline in `server` packages `./services/mcp ./services/events ./services/oauth ./routes`, plus only `TestMcpTokenHashCapAndExpiry` from `./db`. |
| `T1.sh`, `T2.sh` | Require `TestMCPAppsProtocol` and `TestMCPAppsSnapshot`. |
| `T3.sh` | Require `npm run test:unit --` and `npm run build` in `mcp-app`. |
| `T4.sh`, `T5.sh`, `T6.sh` | Require `TestMCPAppsAnswer`, `TestMCPAppsFinalize`, and `TestMCPAppsGroup`. |
| `T7.sh` | Require `npm run test:perf` and `npm run test:e2e`. |
| `F1.sh` through `F6.sh` | Compose protocol; snapshot/read-only; answer; finalize; group; performance/release review gates respectively. |
| `S1.sh` through `S6.sh` | Require the named review-holdout tests in `eval-manifest.json`; weekly/mobile also requires real-host evidence. |
| `run-all.sh` | Execute all task and holdout contracts plus the separate manual host gate. |

`eval-manifest.json` tracks the A1–A12 acceptance IDs from `REQUIREMENTS.md` and maps features exactly: F1 A1–A2, F2 A3–A4, F3 A5–A6, F4 A7–A8, F5 A9–A10, F6 A11–A12; the requirements document is authoritative if its Given/When/Then wording is refined. Implement the named top-level Go tests with meaningful assertions and explicit subtests; never add empty functions simply to satisfy discovery.

The T4/F3 contract requires bounded prepared change-sets applied server-side against a complete own answer, not full-list replacement from a windowed UI. T5/F4 requires actual event-field compare-and-swap predicates and `MatchedCount`, while T6/F5 requires stable create identity, per-target step records, fault injection, and truthful partial/unknown outcomes rather than exactly-once multi-record claims.

Go checks run inside `PROJECT_ROOT/server`: first `go test PACKAGES -list PATTERN`, then parse `go test PACKAGES -json -count=1 -run PATTERN`. Baseline and named Apps suites use the four focused packages above, never broad `go test ./...`; the separate isolated token test runs only its exact name in `./db` and does not run unrelated Mongo tests.

Set `INCLUDE_DB_TESTS=1` only after provisioning an isolated integration fixture to additionally search/run named Apps suites in `./db`. The flag does not provision a database, read credentials, or expand the baseline; missing named integration assertions remain a blocked implementation gap, not grounds to claim cross-record guarantees.

No discovered match, no actual run, no actual pass, or a skipped required named suite is blocked rather than green; the focused baseline permits existing skipped tests only when at least one actual baseline test passes. The isolated token test is mandatory and cannot be skipped.

Npm checks require an actual nonempty script and fail when it exits nonzero or reports no tests. They do not install dependencies, use `--if-present`, or silently choose another script; test scripts must themselves fail on empty suites and use the intended real fixtures, which the human reviewer must inspect.

## Human host evidence

Copy `host-evidence.template.json` to `host-evidence.json` only when preparing to record actual tests. The shipped template deliberately says `not_run` and is never evidence of success.

Record the exact project commit, UI build identifier, named reviewer, timestamps, client versions, OS/browser versions, and real screenshot files for five required host entries: web, Desktop, iOS, Android, and Claude Code text fallback. Cowork is secondary and optional; the Messages API is a non-rendering contract checked by the protocol suite, not a real-account release blocker.

Keep optional host entries at `not_run` unless actual evidence exists; the runner records their status without requiring a paid account or claiming support was tested. For Code, capture actual text-only execution rather than inventing a rendered UI; if API evidence is supplied voluntarily, identify its API version/beta and test-client version in notes.

Screenshot paths are relative to the evidence JSON file and must reference existing PNG, JPEG, or WebP files. Capture only sanitized test data and keep credentials out of screenshots and logs; the harness records command output but never reads or serializes credential stores or environment variables.

For the overall human attestation, set `method` to `manual_in_client`, provide the actual reviewer and review time, and use this exact acknowledgement only after it is true:

> I personally tested or reviewed the attached real-client evidence; JSON validation alone does not prove testing.

Record observed outcomes and named human review for A12 and every holdout S1–S6. S5 must reference actual iOS and Android tests; A12 review must include permission prompts, light/dark theme changes, keyboard/screen-reader checks, safe areas, responsive layout, denied resources, closed/reopened conversation, and text fallbacks.

The gate checks that evidence is structurally complete, screenshots exist, and the recorded project revision matches a clean checkout. It cannot authenticate who filled in the JSON or prove that a screenshot came from the claimed test: a real person must inspect the artifact contents, suite integrity, outcomes, versions, and authorization before accepting release.

To keep host evidence outside this package, set `HOST_EVIDENCE` to its path from the invocation directory. Do not commit personal-data screenshots or credentials.

## Output and exit codes

Each invocation writes `results/<timestamp>-<pid>/results.json`, command logs, and `results/latest.json`. Environment/tool/dependency availability errors remain `blocked`, assertions or invalid output are `failed`, and genuine successful checks are `passed`; command timeout is a failed check, not an excuse to assume success.

The aggregate state is only `not_ready` or `ready_for_human_release_review`. Only a complete T0–T7 plus T1.5 and S1–S6 run with host, owner-decision and live-evidence gates passed can reach the latter, and it never authorizes deployment.

- **Exit 0:** All selected checks passed; a partial selection still reports `not_ready`.
- **Exit 1:** At least one check failed, an unknown gate was requested, or harness processing failed.
- **Exit 2:** Checks are blocked by missing prerequisites or evidence, with no failed check.

Use `run-all.sh` for the release-review decision, not a successful partial wrapper; `run.sh all` remains an equivalent compatibility entry point. This harness contains no deployment commands and performs no application implementation.

The `read-only` selector runs T0, T1, T2, T3, and T7 without requiring unimplemented writes. It remains `not_ready` even if every selected check passes: it is implementation-slice evidence, not the full host/manual release gate or authority to deploy.

## Harness validation versus product validation

Use `bash -n` on every shell entry point, `node --check runner.mjs`, and parse `eval-manifest.json` and `host-evidence.template.json` before running gates. Running with no `PROJECT_ROOT` is a safe negative check: it must emit `not_ready`, blocked checks and exit 2.

Those checks validate only the harness. No real-client screenshot, manual review, performance result, new application test execution, or release readiness is supplied by this handoff.

## F7 text-only amendment

`run.sh text-only` selects T0, T1 and T1.5 without any App renderer, npm UI build, or manual host-UI gate; T1 is the protocol contract, not an instruction to render an App. The existing `read-only` selection is preserved unchanged, and `run.sh read-only T1.5` adds the new text checks without deleting any old gate.

T1.5 requires `TestMCPFieldFindings`, `TestMCPTextParity` and `TestMCPTextEfficiency`. The field suite must emit actual direct Go subtests `FE1`–`FE13` and `W1`–`W8`; parity must emit `REQ_TXT_001`–`REQ_TXT_017`, mapped sequentially to A13–A29 and F7, and efficiency requires `REQ_EFF_001`–`REQ_EFF_006`.

Discovery and a passing parent are insufficient: every required direct child must have actual run/pass events, and any skipped child or descendant blocks the suite. Missing tests, no-test discovery, omitted fixtures, and policy skips remain visible rather than becoming green.

### Owner decisions

Copy `owner_decisions.template.json` to `owner_decisions.json` to collect actual owner decisions, or use the explicit `OWNER_DECISIONS` path. All six entries start pending: `owner_as_invitee`, `omitted_lists`, `list_status_default`, `text_write_concurrency`, `write_lifecycle` and `weekly_shorthand_clock`.

A confirmed entry needs an explicit `selected_option` from `OWNER_DECISIONS.md`, the owner's actual resolution, confirmer, timestamp, approval reference, and `confirmation_source:"human_owner"`. The overall file `status` stays pending until the owner confirms the completed set; proposed defaults are not approval, and unrelated old checks may still run while affected assertions and full release remain blocked.

Choosing lifecycle locks additionally requires `policy_details.reopen_rule` and `affected_operations` text; choosing a named local weekly clock requires `iana_zone_rule`, `reference_date_conversion` and `dst_gap_and_ambiguity_behavior` text. Both are explicit owner-defined policies, not permission for the coding agent to fill in unstated semantics.

### Live evidence produced by the named suites

The runner passes `MUSTER_EVAL_RUN_ID`, `MUSTER_EVAL_STAGE`, `MUSTER_EVAL_STARTED_AT`, `MUSTER_TEXT_EFFICIENCY_PATH`, `MUSTER_FIELD_REGRESSION_PATH` and `MUSTER_OWNER_DECISIONS_PATH` to subprocesses. The named Go suites must write real current-run observations to those output paths, never copy a prefilled passing fixture; templates contain null candidate measurements, null `release_stage` and `not_run` statuses.

The runner derives `release_stage:"text-only"` for T1.5/F7/text-only and incomplete selections, or `"full-build"` for the complete all-gates scope. Both evidence artifacts must match this runner value; an environment override or self-declared text-only artifact cannot relax a full-build run.

The field-regression artifact records newly executed baseline and candidate Go JSON events, the pinned baseline commit, both observation times, and identical test-source SHA256 values. Changed FE1–FE10/FE12/FE13 need same-assertion baseline red/candidate green; FE9 instead needs baseline/candidate green preservation if the owner explicitly keeps `validation_error`, and stays blocked while the choice is pending.

W1–W8 require preservation evidence, while FE11 remains policy-dependent and FE5 must cover the lost text error branch without deleting already-existing structured fields. An earlier diagnostic overlay using a similarly named parent suite is not the required 21-child acceptance suite and is not shipped here.

The current field baseline is `cd089433aa25eea7231047774e6235da3348c44d`, superseding the original `59b1a48b` source review. FE8 now explicitly preserves existing `skipped:string[]` and tests additive `skipped_details` reasons/input indexes; FE9 does not claim the current omitted-both-list descriptions disagree with the validation branch, and FE10 does not claim the current rounding descriptions are inaccurate.

No test files are injected, no baseline checkout is mutated, and no Mongo is provisioned by this harness. Implementers must arrange isolated real fixtures and same-test baseline execution; a declared hash or timestamp is not independent proof, so reviewers must inspect the actual commands, sources and provenance.

### Efficiency evidence format and counting

`text-efficiency.json` must identify `TestMCPTextEfficiency`, current run/commit, fixture/runtime, actual observation time and `observation_mode:"live_test_capture"`. Each named case contains an ordered `records` array; every record has `origin:"model"` or `"bridge"`, a current-run `observed_at`, and `request_raw`/`response_raw` containing the exact serialized JSON-RPC pair with a matching ID.

Counts are recomputed from raw records, not trusted from asserted totals; every model business call counts, including errors, retries, resources/read and hidden extra reads. Only initial `initialize`/`tools/list` belong in separate `setup_records` with `setup_shared_amortized:true`; App-bridge calls are counted separately and cannot replace required model-task traces, and total observed rolling-minute calls must remain below the existing 60-call bucket.

Designated compact reference tasks allow one model call for known-event availability, rank, status, create, finalize-confirm, rename and archive, or two for unknown-event availability. Finalize-confirm must finish with an authoritative success receipt without a second read; response text and structured summaries in these reference tasks are capped at 2,000 Unicode code points, not UTF-16 units.

This cap does not remove legacy full fields or pretend every lossless snapshot fits 2,000 characters. Separate large-response and paginated-response fixtures cap each text/structured page at 20,000 code points, and new summaries/metadata retain the underlying authorization and privacy constraints; privacy and task correctness remain named-suite assertions, not size checks.

The pagination fixture supplies `pagination.request_cursor_pointer` and `pagination.response_cursor_pointer`, using JSON pointers into the actual raw request/response objects. The validator follows those fields through at least two pages, checks each request consumes the previous cursor, rejects repeated cursors, and requires the final captured page to have no next cursor; empty success objects do not count as large-data evidence.

Candidate catalog evidence is the exact compact complete JSON-RPC `tools/list` response with numeric ID 1 for the enabled release stage, never a model-filtered subset. Text-only permits exactly the named 16 base tools, those 16 plus `muster_open_view` (17), or the full 20 when all three write helpers actually exist; partial 18/19-tool helper registrations and unknown/duplicate/substituted names fail.

Full-build review still requires all 20 tools and the helper commit-validation gate. Whenever helpers are present their descriptions must explicitly say “App only”; the opener is the fourth added tool, not an App-only helper, and both stages retain the same byte budgets and baseline-growth alarm.

`helper_validation.mode` is `absent_not_callable` for the 16/17-tool stage, with one current-run failed raw `tools/call` probe for each absent helper (the record `case` is its tool name). For a 20-tool catalog it is `present_guarded`, with `missing_action_id`, `invalid_action_id` and `wrong_actor_action_id` commit probes; each has raw request/response, `observed_at` and `observed_side_effects:false`, and the actor-mismatch case names distinct `caller_fixture` and `prepared_owner_fixture` identifiers.

These are isolated security probes, reported separately from the eight reference task traces; REQ_EFF_006 must assert they actually exercise missing versus actor-bound prepared state. The verified current-tip catalog at `cd089433aa25eea7231047774e6235da3348c44d` is 12,666 UTF-8 bytes across 16 tools, measured locally by the code-review agent on November 2, 2026; it is not a connected-host measurement or approved launch baseline.

The historical measurement at `59b1a48b21060946c6ee11092f6a7d27b113bef7` remains recorded as 12,472 bytes and 16 tools, 194 bytes smaller than the current tip. Future candidate values and human approvals remain null; the transition template now starts from the verified current measurement rather than silently approving either baseline.

Read-only/text-only validation does not invent a successful write to satisfy these negative probes. In the full build, positive actor-bound prepared-action execution is covered by the F3 gate and required REQ_EFF_006 assertions, so an all-negative, nonfunctional helper dispatcher cannot establish full readiness.

The proposed soft target is 24 KiB, hard cap 32 KiB, and growth above 10% against an explicitly approved measured baseline blocks. A first amended-release baseline transition needs an explicit human approval recording old/new commits and byte counts, reason and approval reference; the validator never silently rebases to the candidate or treats the historical measurement as approval.

`selfcheck-f7.mjs` exercises only synthetic negative parser fixtures, such as parent-only success, skipped descendants, missing live evidence and hidden reads. It executes no product suites, fabricates no passing product metrics and writes no release results.
