# Handoff validation

Validated on 2 November 2026. This file distinguishes work completed during research from gates that depend on future implementation.

The sections below retain earlier validation history; the final applicable code pin is cd089433, described in the final branch-drift section. Historical measurements and checks are not relabeled as observations of the newer tip.

## Completed

- **Repository pin:** Development branch tip rechecked at `59b1a48b21060946c6ee11092f6a7d27b113bef7`; inspected worktree remains clean.
- **Current Go baseline:** `services/mcp`, `services/events`, `services/oauth`, and `routes` passed; the isolated `db.TestMcpTokenHashCapAndExpiry` passed.
- **Earlier regression cases:** All eight `TestReviewRegression` cases passed on the current branch.
- **Harness baseline execution:** T0 discovered and executed the focused Go baseline and isolated token test successfully. Its overall report correctly remained `not_ready` because only a baseline slice ran.
- **Specification checker:** 13 critical, 6 high, and 4 medium structural checks passed.
- **Schema validation:** All three Draft 2020-12 schemas compiled successfully with Ajv CLI and explicit format support after the final event-ID/null correction.
- **Script validation:** Node syntax and all shell entrypoints passed syntax checks.
- **Traceability:** The 12 acceptance statements match the manifest; every listed task, acceptance, and holdout script exists.
- **Negative harness behavior:** Missing prerequisites and absent host evidence remain blocked rather than green; the template contains no fabricated screenshots or host results.
- **Cross-review:** Separate code, protocol, and Claude-host reviews informed the final compatibility, null, date-mode, byte-limit, and concurrency corrections.

The schema compiler was run in an isolated transient tool environment, not installed into the Muster repository. Compilation proves schema validity, not server conformance, authorization, or semantic timestamp/CAS behavior.

## Not completed

No MCP App has been built, rendered, performance-tested, or deployed. No new mutation path, actual Mongo conditional-write behavior, live uniqueness/index state, real-host permission flow, or production image has been verified by this handoff.

The shipped evals are an execution contract for the building agent. They require actual implementation tests and real-client evidence before a full run can report readiness for human release review; even that status is not deployment authorization.

## Evidence packaging

The handoff archive excludes temporary command logs, sandbox paths, generated results, credentials, dependency caches, and screenshots. Baseline results are summarized here; the building agent must generate its own implementation evidence against its actual commit.

All source-derived research claims are linked inline in the relevant documents. Architecture choices and performance budgets are labeled as recommendations or project requirements rather than platform guarantees.

## Full-build amendment validation

The initial amendment audit rechecked 59b1a48b, and its focused Go baseline and isolated token test passed again. Nine temporary overlay diagnostic cases checked behavior at that pin and measured its catalog at 12,472 UTF-8 bytes; these historical local observations are not the proposed 21-child acceptance suite or a replay of the user's live 33/34 checks.

Preservation checks confirmed that all original requirement IDs, T0–T7/F1–F6/S1–S6 gates, A1–A12 wording, and original seven decision records remain. F7 adds 17 requirement tests, A13–A29, six efficiency requirements, T1.5, and explicit owner-policy/field-regression/measurement gates.

All eight JSON schemas, including the final-tip skip-string compatibility contract, compiled successfully in the final Draft 2020-12/format validation run. Shell/Node syntax and negative harness selfchecks passed, covering rejection of parent-only test passes, missing/skipped children, null/stale evidence, unapproved policies, hidden calls, and a text-only stage masquerading as full-build evidence.

New application behavior is not implemented and its acceptance tests have not run. Candidate catalog size, response sizes, call counts and future host outcomes remain unmeasured; six owner choices remain pending, so this package is a plan for the coding agent, not a product readiness approval.

### Final branch-drift check

After the initial amendment validation, the branch advanced through cfaffa10 to cd089433. The latter tip was inspected in a separate clean worktree; all focused Go packages and the isolated token test passed there, and catalog size was remeasured at 12,666 bytes/16 tools.

FE-8 and FE-9 dispositions, compatibility decisions, effective regression baseline, and measurement templates were updated accordingly. The shipped skipped string field is preserved, and no claim is made that its reason metadata or the proposed no-op behavior is already implemented.
