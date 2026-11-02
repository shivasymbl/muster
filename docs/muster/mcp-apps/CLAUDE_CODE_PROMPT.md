# Copy-ready handoff for the Muster building agent

You are implementing the approved MCP Apps feature for Muster. Use the attached `Muster-MCP-Apps-Handoff` specification as the control plane, not as a suggestion to redesign the product.

## Baseline and authority

The original research baseline is branch `room/latest-work-completed-s5jq` at `59b1a48b21060946c6ee11092f6a7d27b113bef7`; the final amendment review is pinned to `cd089433aa25eea7231047774e6235da3348c44d`. Fetch/check the current tip, read `CLAUDE.md`, inspect any delta after cd089433, and preserve unrelated changes. The earlier regressions pass; the latest code already has skipped address strings and explicit omitted-list rejection documentation, so do not reintroduce stale findings.

The user approved all three workflows, with a read-only release first and confirmed writes later. You may implement and test in the authorized repository workflow; do not deploy, publish a connector, send real invitations, mutate production data, close #11/#26, or run destructive migrations without separate approval.

This handoff is now the full-build amendment: T0–T7 remain mandatory, and F7/T1.5 plus REQ-EFF-001–006 are added. Read AMENDMENT_CHANGELOG and OWNER_DECISIONS before implementation; the six proposed policy defaults are not approvals, and affected tests/full readiness remain blocked until the owner records choices.

Read in order: README, RESEARCH, REQUIREMENTS, UX_AND_CONTRACTS, IMPLEMENTATION_PLAN, DECISIONS, SYSTEM_MAP, contracts, and evals README. Treat actual current code and primary linked specifications as evidence; surface a concrete conflict rather than silently changing scope.

## Build

Keep Go MCP, the existing website, OAuth, conversational tools, Markdown skills, and GET405/no-notification design. Add one `muster_open_view`, a self-contained hashed `ui://` HTML resource, and an isolated TypeScript App using exact browser SDK `@modelcontextprotocol/ext-apps@1.7.5`.

Use the existing resolved frontend-dist path for compiled App assets. Do not import website auth, Vuex/router bootstrap, analytics, provider calendars, or the old browser-extension bridge into the App. Use the host SDK bridge, not direct browser fetches with a bearer token.

Implement T0–T3 first and verify a real read-only App in Claude web/Desktop. The UI must support ranked comparison, an accessible grid/list, complete own-answer inspection, group participant status, themes, narrow layouts, privacy states, and useful text fallback.

After T1, implement T1.5 before or alongside T2/T3 so the text path delivers value without waiting for an App renderer. It includes stored-answer/update/finalize echoes, shared ranged replace/merge with conditional target writes, default windows, parseable errors, complete parameter documentation, additive event fields/counts/timezone, skipped recipients, filters, weekly shorthand, and explicit lifecycle/owner policies.

Use one range engine and validator set for App and text. Lift shared pure helpers and guarded storage early; do not copy App UI/action-plan dependencies into the conversational call or substitute an unconditional writer for the new range path. Preserve all legacy meanings/defaults except approved, explicitly documented corrections.

Then implement T4–T6 one action at a time. Use the bounded prepare/commit/receipt contract, server-side conditional target writes, complete-answer change-sets, and explicit partial/unknown outcomes. Do not call unconditional legacy writers after a superficial freshness check or claim cross-document atomicity/exactly-once completion.

Keep action flags off until their tests pass. Preserve off-window answer data, manual/calendar layers, current ownership checks, and truthful mail/calendar consequences. Creation/removal requires its own partial-failure and duplicate-prevention evidence.

## Verification and finish line

The `evals/` package contains executable gates, not completed product tests. Implement the named tests with real assertions; missing suites and host evidence must remain blocked, never bypassed with empty tests, skips, fabricated screenshots, or `--if-present`.

Add TestMCPFieldFindings with FE1–FE13/W1–W8, TestMCPTextParity with all 17 direct requirement subtests, and TestMCPTextEfficiency with all six efficiency subtests. Capture same-assertion baseline-red/candidate-green evidence for changed findings, preserve working behaviors, and handle FE9 according to the approved no-op/error choice; FE5 already has structured codes and needs text representation/repair detail, not a fabricated missing-code assertion.

Measure the complete catalog and eight text reference tasks with actual traces, all retries/reads counted, separate bridge calls, and no invented token measurements. The final tip measures 12,666 UTF-8 bytes/16 tools (historical 59b1a48b was 12,472); the future 20-tool size and response budgets are unmeasured until you run them. Preserve the existing UI limits and use the new proposed 24/32 KiB catalog and 2,000/20,000-character text gates, with explicit measured-baseline approval.

Preserve the latest `skipped:string[]` field and add `skipped_details` for reason/index metadata instead of changing its type. FE9's error/documentation agreement is now satisfied; changing it to success no-op still requires D2, while FE10's duration-rejection requirement remains a deliberate behavior change.

Run focused existing Go tests, new protocol/privacy/action tests, actual isolated Mongo conditional-write tests for writes, browser unit/mock-host tests, bundle/performance checks, and real-host scenarios. Run:

```sh
PROJECT_ROOT=/absolute/path/to/repo bash path/to/evals/run.sh read-only
PROJECT_ROOT=/absolute/path/to/repo bash path/to/evals/run.sh text-only
PROJECT_ROOT=/absolute/path/to/repo bash path/to/evals/run-all.sh
```

Commit the approved specification separately from implementation if commits are within your authorized workflow. Keep each implementation commit bounded to the task and include the verification result.

Finish by reporting the current SHA, files changed, feature/action flags, every test actually executed, real-host evidence and untested hosts, measured budgets, remaining limitations, migrations needing approval, and deployment steps awaiting authorization. “Ready for human release review” is the strongest status this work may claim without separately verified deployment.

If blocked, identify the exact failing requirement, reproduction, smallest proposed solution, and the decision needed. Do not replace a blocker with an unrelated event-stream, metrics, mail, authentication, or infrastructure project.
