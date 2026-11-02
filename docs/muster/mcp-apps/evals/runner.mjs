#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import crypto from 'node:crypto';
import { requiredSubtests, ownerDecisions, textEfficiency, fieldRegression } from './f7-evidence.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const manifest = JSON.parse(fs.readFileSync(path.join(here, 'eval-manifest.json'), 'utf8'));
const requested = process.argv.slice(2);
if (!requested.length) requested.push('all');
const allIds = [...Object.keys(manifest.tasks), ...Object.keys(manifest.holdouts)];
const selected = new Set();
const checkSet = new Set();
const selectionErrors = [];
function expand(id) {
  if (id === 'all') {
    allIds.forEach(expand);
    checkSet.add('host:manual');
    return;
  }
  if (id === 'read-only') {
    ['T0', 'T1', 'T2', 'T3', 'T7'].forEach(expand);
    return;
  }
  if (id === 'text-only') {
    ['T0', 'T1', 'T1.5'].forEach(expand);
    return;
  }
  if (selected.has(id)) return;
  const entry = manifest.tasks[id] ?? manifest.features[id] ?? manifest.holdouts[id];
  if (!entry) {
    selectionErrors.push(`Unknown gate: ${id}`);
    return;
  }
  selected.add(id);
  (entry.requires ?? []).forEach(expand);
  (entry.checks ?? []).forEach(check => checkSet.add(check));
}
requested.forEach(expand);
const completeScope = allIds.every(id => selected.has(id)) && checkSet.has('host:manual');
const releaseStage = completeScope ? 'full-build' : 'text-only';
const runId = new Date().toISOString().replace(/[:.]/g, '-') + `-${process.pid}`;
const outDir = path.join(here, 'results', runId);
fs.mkdirSync(outDir, { recursive: true });
const startedAt = Date.now();
const ownerDecisionsPath = process.env.OWNER_DECISIONS
  ? path.resolve(process.cwd(), process.env.OWNER_DECISIONS) : path.join(here, 'owner_decisions.json');
const efficiencyPath = path.join(outDir, 'text-efficiency.json');
const regressionPath = path.join(outDir, 'field-regression.json');
let root;
let rootError;
if (!process.env.PROJECT_ROOT?.trim()) {
  rootError = 'PROJECT_ROOT must be explicitly set to the project checkout path.';
} else {
  root = path.resolve(here, '..', process.env.PROJECT_ROOT);
  if (!fs.existsSync(root) || !fs.statSync(root).isDirectory()) {
    rootError = 'PROJECT_ROOT does not resolve to an existing directory.';
  }
}
const results = [];
const timeout = 15 * 60 * 1000;
let commandIndex = 0;
function command(executable, args, cwd) {
  const executed = spawnSync(executable, args, {
    cwd, encoding: 'utf8', timeout, maxBuffer: 64 * 1024 * 1024,
    // No credential inspection, collection, or environment serialization.
    env: {
      ...process.env,
      MUSTER_EVAL_RUN_ID: runId,
      MUSTER_EVAL_STAGE: releaseStage,
      MUSTER_EVAL_STARTED_AT: new Date(startedAt).toISOString(),
      MUSTER_TEXT_EFFICIENCY_PATH: efficiencyPath,
      MUSTER_FIELD_REGRESSION_PATH: regressionPath,
      MUSTER_OWNER_DECISIONS_PATH: ownerDecisionsPath
    }
  });
  const stem = `${++commandIndex}-${executable.replace(/[^a-z0-9-]/gi, '_')}`;
  fs.writeFileSync(path.join(outDir, `${stem}.stdout.log`), executed.stdout ?? '');
  fs.writeFileSync(path.join(outDir, `${stem}.stderr.log`), executed.stderr ?? '');
  return {
    command: [executable, ...args],
    exit_code: executed.status,
    signal: executed.signal,
    error_code: executed.error?.code ?? null,
    stdout: executed.stdout ?? '',
    stderr: executed.stderr ?? '',
    logs: [`${stem}.stdout.log`, `${stem}.stderr.log`]
  };
}
function brief(cmd) {
  const { stdout, stderr, ...rest } = cmd;
  return rest;
}
function unavailable(cmd) {
  return cmd.error_code === 'ENOENT' ||
    /network is unreachable|no such host|proxyconnect tcp|ENETUNREACH|EAI_AGAIN|ENOTFOUND|Cannot find (?:module|package)|command not found|MODULE_NOT_FOUND|could not read Username|authentication required|permission denied.*(?:cache|go-build)|go: downloading.*\n.*(?:timeout|refused)/i.test(cmd.stderr + cmd.stdout);
}
function failedCommand(cmd, prefix) {
  return {
    status: unavailable(cmd) ? 'blocked' : 'failed',
    reason: `${prefix}; inspect recorded logs. Environment failures are not passes.`,
    commands: [brief(cmd)]
  };
}
function goCheck(name) {
  const server = path.join(root, 'server');
  if (!fs.existsSync(path.join(server, 'go.mod'))) {
    return { status: 'blocked', reason: 'PROJECT_ROOT/server/go.mod is missing; PROJECT_ROOT must be the repository root.' };
  }
  const baseline = name === 'baseline';
  const pattern = baseline ? '^Test' : `^${name}$`;
  const tokenTest = name === 'TestMcpTokenHashCapAndExpiry';
  const packages = tokenTest ? ['./db'] : [...manifest.go_packages];
  // Opt in only when an isolated integration fixture has actually been provisioned.
  // Baseline never broadens to Mongo-dependent db tests.
  if (!baseline && !tokenTest && process.env.INCLUDE_DB_TESTS === '1') packages.push('./db');
  const listed = command('go', ['test', ...packages, '-list', pattern], server);
  if (listed.exit_code !== 0) return failedCommand(listed, 'Go test discovery did not complete');
  const discovered = listed.stdout.split(/\r?\n/).filter(line =>
    baseline ? /^Test\S+$/.test(line) : line === name
  );
  if (!discovered.length) {
    return {
      status: 'blocked', reason: `No matching Go tests discovered for ${pattern}; planned implementation is missing.`,
      commands: [brief(listed)], discovered_tests: []
    };
  }
  const tested = command('go', ['test', ...packages, '-json', '-count=1', '-run', pattern], server);
  const events = [];
  let parseError = false;
  for (const line of tested.stdout.split(/\r?\n/).filter(Boolean)) {
    try { events.push(JSON.parse(line)); } catch { parseError = true; }
  }
  const matching = ev => ev.Test && !ev.Test.includes('/') &&
    (baseline ? /^Test/.test(ev.Test) : ev.Test === name);
  const runs = events.filter(ev => ev.Action === 'run' && matching(ev));
  const passes = events.filter(ev => ev.Action === 'pass' && matching(ev));
  const skips = events.filter(ev => ev.Action === 'skip' && matching(ev));
  const failures = events.filter(ev => ev.Action === 'fail');
  const detail = {
    commands: [brief(listed), brief(tested)],
    discovered_tests: [...new Set(discovered)],
    executed_tests: runs.map(ev => `${ev.Package}:${ev.Test}`),
    passed_test_count: passes.length,
    skipped_test_count: skips.length
  };
  if (tested.exit_code !== 0) {
    return { ...failedCommand(tested, 'Go suite did not pass'), ...detail };
  }
  if (parseError || failures.length) {
    return { status: 'failed', reason: 'Invalid Go JSON or failing test/package event.', ...detail };
  }
  if (!runs.length || !passes.length || (!baseline && (skips.length || passes.length !== runs.length))) {
    return { status: 'blocked', reason: 'Zero executed/passed tests or skipped required suite; refusing a false green.', ...detail };
  }
  const names = manifest.required_subtests?.[name];
  if (names) {
    const checked = requiredSubtests(events, name, names);
    detail.required_subtests = checked.subtests;
    if (checked.issues.length) {
      return { status: 'blocked', reason: 'Required direct subtests or their nested assertions did not all execute and pass.', issues: checked.issues, ...detail };
    }
  }
  return { status: 'passed', reason: 'Discovered matching tests and verified actual successful execution using go test -json.', ...detail };
}
function npmCheck(script) {
  const ui = path.join(root, 'mcp-app');
  const packagePath = path.join(ui, 'package.json');
  if (!fs.existsSync(packagePath)) return { status: 'blocked', reason: 'mcp-app/package.json does not exist.' };
  let pkg;
  try { pkg = JSON.parse(fs.readFileSync(packagePath, 'utf8')); }
  catch { return { status: 'failed', reason: 'mcp-app/package.json is invalid JSON.' }; }
  if (typeof pkg.scripts?.[script] !== 'string' || !pkg.scripts[script].trim()) {
    return { status: 'blocked', reason: `Required npm script ${script} is missing; no fallback or --if-present is allowed.` };
  }
  const args = ['run', script];
  if (script === 'test:unit') args.push('--');
  const tested = command('npm', args, ui);
  if (tested.exit_code !== 0) return failedCommand(tested, `npm ${script} did not pass`);
  if (script.startsWith('test:') && /no tests found|\b0\s+tests?\b|tests?\s*:\s*0\b/i.test(tested.stdout + tested.stderr)) {
    return { status: 'failed', reason: 'Test runner reports zero tests; configure failure on empty suites.', commands: [brief(tested)] };
  }
  return {
    status: 'passed',
    reason: 'Required npm script exists and exited successfully; human reviewer must verify script and fixture integrity.',
    commands: [brief(tested)]
  };
}
function realText(value) {
  return typeof value === 'string' && value.trim().length >= 3 &&
    !/^(?:not_run|unknown|pending|todo|tbd|example|your[- _].*|<.*>)$/i.test(value.trim());
}
function pastDate(value) {
  return typeof value === 'string' && Number.isFinite(Date.parse(value)) &&
    Date.parse(value) <= Date.now() + 5 * 60 * 1000;
}
function screenshot(file, evidenceDir) {
  if (typeof file !== 'string' || path.isAbsolute(file)) return null;
  const resolved = path.resolve(evidenceDir, file);
  if (!fs.existsSync(resolved) || !fs.statSync(resolved).isFile()) return null;
  const bytes = fs.readFileSync(resolved);
  const png = bytes.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]));
  const jpeg = bytes[0] === 255 && bytes[1] === 216 && bytes[2] === 255;
  const webp = bytes.toString('ascii', 0, 4) === 'RIFF' && bytes.toString('ascii', 8, 12) === 'WEBP';
  if (bytes.length < 100 || !(png || jpeg || webp)) return null;
  return { file, bytes: bytes.length, sha256: crypto.createHash('sha256').update(bytes).digest('hex') };
}
function hostCheck() {
  const evidencePath = process.env.HOST_EVIDENCE
    ? path.resolve(process.cwd(), process.env.HOST_EVIDENCE)
    : path.join(here, 'host-evidence.json');
  if (!fs.existsSync(evidencePath)) {
    return { status: 'blocked', reason: 'host-evidence.json missing. A real person must test the clients and review screenshots; template is not evidence.' };
  }
  let evidence;
  try { evidence = JSON.parse(fs.readFileSync(evidencePath, 'utf8')); }
  catch { return { status: 'failed', reason: 'Host evidence is not valid JSON.' }; }
  const issues = [];
  const attestation = evidence.human_review ?? {};
  if (attestation.reviewed !== true || !realText(attestation.reviewer) ||
      !pastDate(attestation.reviewed_at) || attestation.method !== 'manual_in_client' ||
      attestation.acknowledgement !== 'I personally tested or reviewed the attached real-client evidence; JSON validation alone does not prove testing.') {
    issues.push('Missing explicit named human-review attestation, valid review date, or manual-in-client method.');
  }
  if (evidence.schema_version !== 1 || evidence.status !== 'reviewed') {
    issues.push('Evidence must be schema version 1 and reviewed, not not_run.');
  }
  if (!/^[0-9a-f]{40}$/i.test(evidence.project_commit ?? '') || !realText(evidence.ui_build_id)) {
    issues.push('An actual 40-character project commit and UI build identifier are required.');
  } else {
    const revision = command('git', ['rev-parse', 'HEAD'], root);
    if (revision.exit_code !== 0 || revision.stdout.trim() !== evidence.project_commit) {
      issues.push('Evidence project commit does not match current checkout HEAD, or Git is unavailable.');
    }
    const dirty = command('git', ['status', '--porcelain', '--untracked-files=all'], root);
    if (dirty.exit_code !== 0 || dirty.stdout.trim()) {
      issues.push('Host gate requires a clean checkout so screenshot evidence can be tied to the reviewed commit.');
    }
  }
  const verifiedArtifacts = [];
  for (const host of manifest.required_hosts) {
    const entry = evidence.hosts?.[host] ?? {};
    if (entry.status !== 'passed' || !realText(entry.client_version) ||
        !realText(entry.os_or_browser_version) || !realText(entry.reviewer) ||
        !pastDate(entry.tested_at) || !realText(entry.notes)) {
      issues.push(`${host}: passed status, client and OS/browser versions, reviewer, test date and substantive notes required.`);
    }
    const artifacts = (Array.isArray(entry.screenshots) ? entry.screenshots : [])
      .map(file => screenshot(file, path.dirname(evidencePath)));
    if (!artifacts.length || artifacts.some(item => !item)) {
      issues.push(`${host}: at least one existing real screenshot file (PNG/JPEG/WebP) required.`);
    } else {
      verifiedArtifacts.push({ host, artifacts });
    }
  }
  for (const id of manifest.required_manual_scenarios) {
    const scenario = evidence.scenarios?.[id] ?? {};
    if (scenario.status !== 'passed' || !realText(scenario.reviewer) ||
        !pastDate(scenario.reviewed_at) || !realText(scenario.observed_outcome) ||
        !Array.isArray(scenario.hosts) || !scenario.hosts.length ||
        scenario.hosts.some(host => ![...manifest.required_hosts, ...manifest.optional_hosts].includes(host))) {
      issues.push(`${id}: named human review, actual observed outcome, date and valid host references required.`);
    }
    if (id === 'S5' && !['claude_ios', 'claude_android'].every(host => scenario.hosts?.includes(host))) {
      issues.push('S5 must explicitly cover actual iOS and Android clients.');
    }
  }
  return {
    status: issues.length ? 'blocked' : 'passed',
    reason: issues.length
      ? 'Manual host evidence is incomplete or not tied to this build.'
      : 'Evidence structure, files, revision and human attestation validated. This cannot establish that claimed human actions occurred; independent release review remains mandatory.',
    issues, artifacts: verifiedArtifacts,
    optional_host_status: Object.fromEntries(manifest.optional_hosts.map(host =>
      [host, evidence.hosts?.[host]?.status ?? 'not_run']))
  };
}
function readEvidence(file) {
  if (!fs.existsSync(file)) return null;
  return JSON.parse(fs.readFileSync(file, 'utf8'));
}
function liveEvidence(kind) {
  const file = kind === 'efficiency' ? efficiencyPath : regressionPath;
  const evidence = readEvidence(file);
  if (!evidence) return {
    status: 'blocked',
    reason: `${kind}: named test did not generate current-run live evidence. Templates, historical reports and asserted totals cannot pass.`,
    measured: null
  };
  const revision = command('git', ['rev-parse', 'HEAD'], root);
  if (revision.exit_code !== 0) return failedCommand(revision, 'Cannot bind evidence to current project commit');
  const expected = {
    run_id: runId, project_commit: revision.stdout.trim(), started_at: startedAt, release_stage: releaseStage,
    owner_decisions: readEvidence(ownerDecisionsPath)
  };
  return kind === 'efficiency'
    ? textEfficiency(evidence, expected, manifest.text_efficiency)
    : fieldRegression(evidence, expected, manifest.field_regression);
}
for (const check of checkSet) {
  let outcome;
  const started = Date.now();
  if (rootError) outcome = { status: 'blocked', reason: rootError };
  else {
    const separator = check.indexOf(':');
    const kind = check.slice(0, separator);
    const name = check.slice(separator + 1);
    try {
      if (kind === 'go') outcome = goCheck(name);
      else if (kind === 'npm') outcome = npmCheck(name);
      else if (kind === 'host') outcome = hostCheck();
      else if (kind === 'owner') outcome = ownerDecisions(readEvidence(ownerDecisionsPath), manifest.owner_decisions);
      else if (kind === 'text' && name === 'efficiency') outcome = liveEvidence('efficiency');
      else if (kind === 'field' && name === 'regression') outcome = liveEvidence('regression');
      else outcome = { status: 'failed', reason: `Unknown check kind ${kind}.` };
    } catch (error) {
      outcome = { status: 'failed', reason: `Harness exception: ${error.message}` };
    }
  }
  results.push({ check, duration_ms: Date.now() - started, ...outcome });
  console.log(`${check}: ${outcome.status} - ${outcome.reason}`);
}
const ready = completeScope && !selectionErrors.length && results.length > 0 &&
  results.every(result => result.status === 'passed');
const report = {
  schema_version: 1,
  run_id: runId,
  generated_at: new Date().toISOString(),
  state: ready ? 'ready_for_human_release_review' : 'not_ready',
  deployment_authorized: false,
  scope_complete: completeScope,
  release_stage: releaseStage,
  requested_gates: requested,
  expanded_gates: [...selected],
  errors: selectionErrors,
  project_root_configured: Boolean(process.env.PROJECT_ROOT),
  checks: results,
  warning: 'Planned suites must contain genuine assertions. Test exit codes and host JSON are not independent proof of coverage or real human testing. A human must inspect evidence and approve release separately.'
};
fs.writeFileSync(path.join(outDir, 'results.json'), JSON.stringify(report, null, 2) + '\n');
fs.writeFileSync(path.join(here, 'results', 'latest.json'), JSON.stringify(report, null, 2) + '\n');
console.log(`State: ${report.state}`);
console.log(`Results: ${path.join(outDir, 'results.json')}`);
process.exitCode = ready || (completeScope === false && !selectionErrors.length && results.length > 0 &&
  results.every(result => result.status === 'passed')) ? 0 : results.some(r => r.status === 'failed') || selectionErrors.length ? 1 : 2;
