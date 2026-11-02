// Pure validators for planned F7 evidence. They never call MCP, read credentials,
// provision fixtures, or treat a supplied JSON object as independent execution proof.
import crypto from 'node:crypto';

const meaningful = x => typeof x === 'string' && x.trim().length >= 3 &&
  !/^(?:pending|not_run|unknown|todo|tbd)$/i.test(x.trim());
const validDate = x => typeof x === 'string' && Number.isFinite(Date.parse(x)) &&
  Date.parse(x) <= Date.now() + 300000;
const sha256 = x => crypto.createHash('sha256').update(x).digest('hex');
const chars = x => [...x].length;
const commit = x => typeof x === 'string' && /^[0-9a-f]{40}$/i.test(x);

export function requiredSubtests(events, suite, names) {
  const packages = [...new Set(events.filter(e => e.Action === 'run' && e.Test === suite).map(e => e.Package))];
  const issues = [];
  const subtests = [];
  if (!packages.length) issues.push(`${suite}: no parent execution event.`);
  for (const pkg of packages) {
    for (const name of names) {
      const test = `${suite}/${name}`;
      const own = events.filter(e => e.Package === pkg && e.Test === test);
      const descendants = events.filter(e => e.Package === pkg && e.Test?.startsWith(`${test}/`));
      const ran = own.some(e => e.Action === 'run');
      const passed = own.some(e => e.Action === 'pass');
      const skipped = [...own, ...descendants].some(e => e.Action === 'skip');
      const failed = [...own, ...descendants].some(e => e.Action === 'fail');
      const status = failed ? 'failed' : ran && passed && !skipped ? 'passed' : 'blocked';
      subtests.push({ package: pkg, test, status });
      if (status !== 'passed') issues.push(`${pkg}:${test}: missing run/pass, skipped descendant, or failure.`);
    }
  }
  return { issues, subtests };
}

export function ownerDecisions(evidence, definitions) {
  const issues = [];
  const decisions = [];
  if (evidence?.schema_version !== 1) issues.push('Owner decisions schema_version must be 1.');
  if (evidence?.status !== 'confirmed') issues.push('Owner decisions remain pending until the actual owner confirms the completed decision set.');
  for (const [key, definition] of Object.entries(definitions)) {
    const item = evidence?.decisions?.[key];
    const requiredDetails = definition.required_details_by_choice?.[item?.selected_option] ?? [];
    const detailsComplete = requiredDetails.every(field => meaningful(item?.policy_details?.[field]));
    const approved = item?.status === 'confirmed' && meaningful(item.resolution) &&
      meaningful(item.confirmed_by) && validDate(item.confirmed_at) &&
      meaningful(item.approval_reference) && item.confirmation_source === 'human_owner' &&
      (!definition.allowed_choices || definition.allowed_choices.includes(item.selected_option)) && detailsComplete;
    decisions.push({
      key, status: approved ? 'confirmed' : 'pending',
      affected_operations: definition.affected_operations,
      proposed_default_is_approval: false
    });
    if (!approved) issues.push(`${key}: unresolved; an explicit human-owner decision is required. Proposed defaults are not approval.`);
  }
  return {
    status: issues.length ? 'blocked' : 'passed', issues, decisions,
    reason: issues.length ? 'Owner-policy decisions remain unresolved; unrelated gates may still execute.'
      : 'All required owner decisions have explicit confirmation metadata; human release review must verify provenance.'
  };
}

function pair(requestRaw, responseRaw, label, issues) {
  if (typeof requestRaw !== 'string' || typeof responseRaw !== 'string') {
    issues.push(`${label}: exact raw request/response strings required.`);
    return null;
  }
  try {
    const request = JSON.parse(requestRaw), response = JSON.parse(responseRaw);
    if (request.jsonrpc !== '2.0' || response.jsonrpc !== '2.0' ||
        typeof request.method !== 'string' || request.id === undefined ||
        response.id !== request.id ||
        (response.result === undefined) === (response.error === undefined)) {
      issues.push(`${label}: invalid JSON-RPC pair, matching ID, or exclusive result/error.`);
      return null;
    }
    return { request, response };
  } catch {
    issues.push(`${label}: malformed JSON-RPC string.`);
    return null;
  }
}

function catalogue(data, label, issues) {
  const parsed = pair(data?.request_raw, data?.response_raw, label, issues);
  if (!parsed) return null;
  if (parsed.request.method !== 'tools/list' || parsed.request.id !== 1 ||
      data.response_raw !== JSON.stringify(parsed.response) || !Array.isArray(parsed.response.result?.tools) ||
      parsed.response.result.tools.length === 0 || parsed.response.result.nextCursor) {
    issues.push(`${label}: complete nonempty tools/list with compact full JSON-RPC response and id 1 required; do not compare another envelope or one page as the total catalog.`);
    return null;
  }
  return {
    utf8_bytes: Buffer.byteLength(data.response_raw, 'utf8'),
    tool_count: parsed.response.result.tools.length,
    sha256: sha256(data.response_raw)
  };
}

function pointer(document, location) {
  if (typeof location !== 'string' || !location.startsWith('/')) return undefined;
  return location.slice(1).split('/').reduce((value, part) =>
    value?.[part.replace(/~1/g, '/').replace(/~0/g, '~')], document);
}

function liveIdentity(evidence, expected, generatedBy, issues) {
  if (evidence?.schema_version !== 1 || evidence.status !== 'measured' ||
      evidence.generated_by !== generatedBy || evidence.run_id !== expected.run_id ||
      !commit(evidence.project_commit) || evidence.project_commit !== expected.project_commit ||
      !validDate(evidence.measured_at) || Date.parse(evidence.measured_at) < expected.started_at ||
      evidence.observation_mode !== 'live_test_capture') {
    issues.push('Missing current-run, current-commit, live-test-generated evidence; null/template/replayed reports cannot pass.');
  }
  if (!['text-only', 'full-build'].includes(expected.release_stage) || evidence?.release_stage !== expected.release_stage) {
    issues.push('Evidence release_stage must match the runner-derived stage; a full-build run cannot self-declare text-only evidence.');
  }
  if (!meaningful(evidence?.fixture_id) || !meaningful(evidence?.runtime_version)) {
    issues.push('Actual isolated fixture and runtime identifiers are required.');
  }
}

export function catalogStage(tools, stage, budgets) {
  const issues = [];
  const names = tools.map(tool => tool?.name);
  const base = budgets.base_tool_names, opener = budgets.opener_tool;
  const helpers = budgets.app_only_helpers;
  const allowed = new Set([...base, opener, ...helpers]);
  const helperCount = names.filter(name => helpers.includes(name)).length;
  if (new Set(names).size !== names.length || names.some(name => !allowed.has(name)) ||
      base.some(name => !names.includes(name))) {
    issues.push('Catalog must contain every named base tool exactly once and no unknown or duplicate tool names.');
  }
  const validShape = names.length === 16 && !names.includes(opener) && helperCount === 0 ||
    names.length === 17 && names.includes(opener) && helperCount === 0 ||
    names.length === 20 && names.includes(opener) && helperCount === 3;
  if (!validShape) issues.push('Only the exact 16-base, 17-with-opener, or complete 20-tool registration is valid; partial helper sets are not supported.');
  if (!['text-only', 'full-build'].includes(stage)) issues.push('Unknown runner release stage.');
  if (stage === 'full-build' && (names.length !== 20 || helperCount !== 3 || !names.includes(opener))) {
    issues.push('Full-build review requires the complete 20-tool catalog and all prepared-action helpers.');
  }
  for (const name of helpers.filter(name => names.includes(name))) {
    if (!tools.some(t => t?.name === name && typeof t.description === 'string' && /\bApp only\b/i.test(t.description))) {
      issues.push(`${name}: present helper must explicitly say App only.`);
    }
  }
  return { issues, helpers_present: helperCount === 3, tool_count: names.length, release_stage: stage };
}

function helperValidation(evidence, expected, stageCatalog, budgets, issues) {
  const validation = evidence?.helper_validation;
  const present = stageCatalog?.helpers_present === true;
  const expectedMode = present ? 'present_guarded' : 'absent_not_callable';
  const requiredCases = present ? ['missing_action_id', 'invalid_action_id', 'wrong_actor_action_id'] : budgets.app_only_helpers;
  if (validation?.mode !== expectedMode || !Array.isArray(validation?.records)) {
    issues.push(`Actual ${expectedMode} helper-probe evidence is required for this catalog.`);
    return;
  }
  for (const id of requiredCases) {
    const records = validation.records.filter(record => record?.case === id);
    if (records.length !== 1) { issues.push(`helper/${id}: exactly one actual negative probe required.`); continue; }
    const record = records[0];
    const parsed = pair(record.request_raw, record.response_raw, `helper/${id}`, issues);
    if (!parsed) continue;
    if (!validDate(record.observed_at) || Date.parse(record.observed_at) < expected.started_at ||
        record.observed_side_effects !== false) {
      issues.push(`helper/${id}: current-run timestamp and actual no-side-effect observation required.`);
    }
    const tool = present ? 'muster_ui_commit_action' : id;
    if (parsed.request.method !== 'tools/call' || parsed.request.params?.name !== tool ||
        !(parsed.response.error || parsed.response.result?.isError === true)) {
      issues.push(`helper/${id}: raw call must address the actual helper and show a rejected result, not a claimed rejection.`);
    }
    if (!present) {
      const body = parsed.response;
      const messages = [
        body.error?.message, body.result?.structuredContent?.message,
        body.result?.structuredContent?.data?.message,
        ...(Array.isArray(body.result?.content) ? body.result.content.filter(c => c?.type === 'text').map(c => c.text) : [])
      ].filter(x => typeof x === 'string').join('\n');
      if (body.error?.code !== -32601 && !/unknown tool|tool.{0,40}(?:disabled|unavailable|not enabled)|not registered|method not found/i.test(messages)) {
        issues.push(`helper/${id}: rejecting missing arguments is not proof of absence; capture the actual unknown/disabled-helper rejection.`);
      }
    }
    const args = parsed.request.params?.arguments;
    if (present && id === 'missing_action_id' && args && Object.hasOwn(args, 'action_id')) {
      issues.push('helper/missing_action_id: probe must actually omit action_id.');
    }
    if (present && id !== 'missing_action_id' && !meaningful(args?.action_id)) {
      issues.push(`helper/${id}: probe must actually supply its test action_id.`);
    }
    if (present && id === 'wrong_actor_action_id' &&
        (!meaningful(record.caller_fixture) || !meaningful(record.prepared_owner_fixture) ||
         record.caller_fixture === record.prepared_owner_fixture)) {
      issues.push('helper/wrong_actor_action_id: distinct actual caller/prepared-owner fixture identities required.');
    }
  }
}

export function textEfficiency(evidence, expected, budgets) {
  const issues = [], warnings = [], measuredCases = [];
  let stageCatalog = null;
  liveIdentity(evidence, expected, 'TestMCPTextEfficiency', issues);
  const measuredCatalog = catalogue(evidence?.catalog, 'current catalog', issues);
  const baseline = evidence?.approved_baseline;
  const measuredBaseline = catalogue(baseline?.catalog, 'approved baseline catalog', issues);
  if (baseline?.status !== 'approved' || !commit(baseline.project_commit) ||
      !meaningful(baseline.approved_by) || !validDate(baseline.approved_at) ||
      !meaningful(baseline.approval_reference) || !validDate(baseline.measured_at) ||
      baseline.observation_mode !== 'live_baseline_capture') {
    issues.push('An explicitly human-approved, actually measured baseline catalog is required; a proposed byte value is not evidence.');
  }
  const observed = budgets.observed_legacy_catalog;
  if (measuredBaseline && (baseline?.project_commit !== observed.project_commit ||
      measuredBaseline.utf8_bytes !== observed.utf8_bytes)) {
    const transition = baseline?.transition;
    if (transition?.status !== 'approved' || transition.from_commit !== observed.project_commit ||
        transition.from_utf8_bytes !== observed.utf8_bytes ||
        transition.to_commit !== baseline.project_commit ||
        transition.to_utf8_bytes !== measuredBaseline.utf8_bytes ||
        !meaningful(transition.approved_by) || !validDate(transition.approved_at) ||
        !meaningful(transition.reason) || !meaningful(transition.approval_reference)) {
      issues.push('Replacing the measured legacy baseline requires an explicit human-approved measured transition; auto-rebaseline is forbidden.');
    }
  }
  if (measuredCatalog) {
    const currentTools = JSON.parse(evidence.catalog.response_raw).result.tools;
    stageCatalog = catalogStage(currentTools, expected.release_stage, budgets);
    issues.push(...stageCatalog.issues);
    helperValidation(evidence, expected, stageCatalog, budgets, issues);
    if (measuredCatalog.utf8_bytes > budgets.catalog_hard_bytes) issues.push('Catalog exceeds hard UTF-8 byte cap.');
    else if (measuredCatalog.utf8_bytes > budgets.catalog_soft_bytes) warnings.push('Catalog exceeds soft UTF-8 byte target.');
    if (measuredBaseline && measuredCatalog.utf8_bytes > measuredBaseline.utf8_bytes * (1 + budgets.catalog_regression_fraction)) {
      issues.push('Catalog exceeds the allowed regression against the approved measured baseline.');
    }
  }
  const cases = Array.isArray(evidence?.cases) ? evidence.cases : [];
  const callTimes = [];
  const expectedIds = new Set(Object.keys(budgets.cases));
  if (cases.some(c => !expectedIds.has(c?.id)) || new Set(cases.map(c => c?.id)).size !== cases.length) {
    issues.push('Unexpected or duplicate efficiency case IDs.');
  }
  for (const [id, budget] of Object.entries(budgets.cases)) {
    const found = cases.filter(c => c?.id === id);
    if (found.length !== 1) { issues.push(`${id}: exactly one live case required.`); continue; }
    const item = found[0], records = Array.isArray(item.records) ? item.records : [];
    if (budget.required_actor_role && item.actor_role !== budget.required_actor_role) {
      issues.push(`${id}: the designated ${budget.required_actor_role} reference fixture is required.`);
    }
    if (item.status !== 'completed' || !meaningful(item.observed_outcome) || !records.length) {
      issues.push(`${id}: completed observed outcome and raw call records required.`);
    }
    let model = 0, bridge = 0;
    const measurements = [], seenIds = new Set();
    for (const [index, record] of records.entries()) {
      if (!validDate(record?.observed_at) || Date.parse(record.observed_at) < expected.started_at) {
        issues.push(`${id}/${index}: call requires a current-run observation timestamp.`);
      } else callTimes.push(Date.parse(record.observed_at));
      if (record.origin === 'model') model++;
      else if (record.origin === 'bridge') bridge++;
      else issues.push(`${id}/${index}: origin must explicitly be model or bridge.`);
      const parsed = pair(record.request_raw, record.response_raw, `${id}/${index}`, issues);
      if (!parsed) continue;
      const key = JSON.stringify(parsed.request.id);
      if (seenIds.has(key)) issues.push(`${id}/${index}: duplicate request ID within case.`);
      seenIds.add(key);
      // Every business-method record counts, including failures, retries,
      // resources/read and hidden model reads. Only initial setup is separate.
      if (['initialize', 'tools/list'].includes(parsed.request.method)) {
        issues.push(`${id}/${index}: discovery/setup must be recorded in setup_records, not hidden inside or removed from business traces.`);
      }
      const result = parsed.response.result;
      const text = Array.isArray(result?.content)
        ? result.content.filter(c => c?.type === 'text').map(c => c.text).filter(t => typeof t === 'string').join('\n')
        : '';
      const summary = typeof result?.structuredContent?.summary === 'string' ? result.structuredContent.summary :
        typeof result?.structuredContent?.data?.summary === 'string' ? result.structuredContent.data.summary : null;
      const textChars = chars(text), summaryChars = summary === null ? null : chars(summary);
      const structuredChars = result?.structuredContent === undefined ? null : chars(JSON.stringify(result.structuredContent));
      if (textChars > budget.response_chars) issues.push(`${id}/${index}: designated ${budget.response_class} text exceeds Unicode code-point budget.`);
      if (summaryChars !== null && summaryChars > budgets.structured_summary_chars) {
        issues.push(`${id}/${index}: structured summary exceeds Unicode code-point budget.`);
      }
      if (budget.response_class !== 'compact' && structuredChars !== null && structuredChars > budget.response_chars) {
        issues.push(`${id}/${index}: large/page structured response exceeds Unicode code-point budget.`);
      }
      if (budget.response_class !== 'compact' && !text && (structuredChars === null || result?.structuredContent === null) && !parsed.response.error) {
        issues.push(`${id}/${index}: large/page fixture must contain actual response data, not an empty success.`);
      }
      if (budget.response_class === 'compact' && record.origin === 'model' && !text && summary === null && !parsed.response.error) {
        issues.push(`${id}/${index}: compact task lacks a usable text or structured summary.`);
      }
      measurements.push({
        index, origin: record.origin, method: parsed.request.method,
        tool: parsed.request.params?.name ?? null,
        is_error: Boolean(parsed.response.error || result?.isError),
        text_characters: textChars, structured_summary_characters: summaryChars,
        structured_characters: structuredChars,
        request_utf8_bytes: Buffer.byteLength(record.request_raw, 'utf8'),
        response_utf8_bytes: Buffer.byteLength(record.response_raw, 'utf8'),
        wire_utf8_bytes: Buffer.byteLength(record.request_raw, 'utf8') + Buffer.byteLength(record.response_raw, 'utf8'),
        request_sha256: sha256(record.request_raw), response_sha256: sha256(record.response_raw)
      });
    }
    if (!model) issues.push(`${id}: requires an actual model-initiated task trace.`);
    if (budget.model_calls !== null && model > budget.model_calls) issues.push(`${id}: model MCP-call budget exceeded, including failed/retried/read calls.`);
    if (budget.min_records && records.length < budget.min_records) issues.push(`${id}: insufficient raw page records.`);
    if (budget.response_class === 'page') {
      const pagination = item.pagination;
      if (typeof pagination?.request_cursor_pointer !== 'string' ||
          typeof pagination?.response_cursor_pointer !== 'string') {
        issues.push(`${id}: JSON pointers to actual request/response cursor fields are required.`);
      } else {
        let previous;
        const seenCursors = new Set();
        for (const [index, record] of records.entries()) {
          const parsed = pair(record.request_raw, record.response_raw, `${id}/pagination/${index}`, issues);
          if (!parsed) continue;
          const requested = pointer(parsed.request, pagination.request_cursor_pointer);
          const returned = pointer(parsed.response, pagination.response_cursor_pointer);
          if (index === 0 ? requested != null && requested !== '' : requested !== previous) {
            issues.push(`${id}/${index}: actual cursor chain does not start at the first page or follow the prior response.`);
          }
          if (index < records.length - 1) {
            if (typeof returned !== 'string' || !returned.length || seenCursors.has(returned)) issues.push(`${id}/${index}: next cursor is absent, empty or repeated.`);
            seenCursors.add(returned);
          } else if (returned != null && returned !== '') issues.push(`${id}: final captured page still has an unconsumed next cursor.`);
          previous = returned;
        }
      }
    }
    const finalModel = measurements.filter(r => r.origin === 'model').at(-1);
    if (!finalModel || finalModel.is_error) issues.push(`${id}: final model-facing response must be an authoritative success, not a claimed completion after error.`);
    measuredCases.push({
      id, model_mcp_calls: model, app_bridge_calls: bridge,
      total_wire_utf8_bytes: measurements.reduce((sum, r) => sum + r.wire_utf8_bytes, 0),
      records: measurements
    });
  }
  const setup = Array.isArray(evidence?.setup_records) ? evidence.setup_records : null;
  if (!setup) issues.push('setup_records must be an explicit array, even when empty.');
  for (const [index, record] of (setup ?? []).entries()) {
    const parsed = pair(record?.request_raw, record?.response_raw, `setup/${index}`, issues);
    if (parsed && !['initialize', 'tools/list'].includes(parsed.request.method)) {
      issues.push(`setup/${index}: business calls or resource reads cannot be relabeled as excluded setup.`);
    }
    if (!validDate(record?.observed_at) || Date.parse(record.observed_at) < expected.started_at) {
      issues.push(`setup/${index}: current-run observation timestamp required.`);
    } else callTimes.push(Date.parse(record.observed_at));
  }
  if (setup?.length && evidence.setup_shared_amortized !== true) {
    issues.push('Setup is excluded only when explicitly captured as shared/amortized initial discovery.');
  }
  callTimes.sort((a, b) => a - b);
  let maxRollingMinute = 0;
  for (let first = 0, last = 0; last < callTimes.length; last++) {
    while (callTimes[last] - callTimes[first] >= 60000) first++;
    maxRollingMinute = Math.max(maxRollingMinute, last - first + 1);
  }
  if (maxRollingMinute >= 60) issues.push('Observed task/setup/bridge trace reaches the existing 60-call/minute bucket; do not evade throttling or hide retries.');
  return {
    status: issues.length ? 'blocked' : 'passed',
    reason: issues.length ? 'Text efficiency lacks complete valid live evidence or exceeds a proposed gate.'
      : 'Counts and sizes recomputed from current-run raw traces; compact reference cases meet budgets. Human review must verify trace provenance and outcome/privacy assertions.',
    issues, warnings,
    measured: { catalog: measuredCatalog, catalog_stage: stageCatalog, approved_baseline: measuredBaseline, cases: measuredCases, setup_calls: setup?.length ?? null, helper_security_probes: evidence?.helper_validation?.records?.length ?? null, max_observed_calls_in_rolling_minute: maxRollingMinute },
    scope: 'The 2000-character cap covers designated compact reference outputs and structured summaries, not every lossless full snapshot; legacy full fields must remain available with separately bounded large/page responses.'
  };
}

export function fieldRegression(evidence, expected, required) {
  const issues = [];
  const omittedDecision = expected.owner_decisions?.decisions?.omitted_lists;
  const preservedFE9 = omittedDecision?.status === 'confirmed' && omittedDecision.selected_option === 'validation_error';
  const changedFE9 = omittedDecision?.status === 'confirmed' && omittedDecision.selected_option === 'success_noop';
  if (!preservedFE9 && !changedFE9) issues.push('FE9 baseline expectation remains pending D2; do not invent a no-op approval or call existing validation a defect.');
  const redNames = required.red.filter(name => name !== 'FE9' || changedFE9);
  const preservedNames = [...required.preserved, ...(preservedFE9 ? ['FE9'] : [])];
  liveIdentity(evidence, expected, 'TestMCPFieldFindings', issues);
  if (!commit(evidence?.baseline_commit) || evidence.baseline_commit !== required.baseline_commit ||
      evidence.baseline_commit === evidence.project_commit ||
      !/^[0-9a-f]{64}$/i.test(evidence?.test_source_sha256 ?? '') ||
      evidence.baseline_test_source_sha256 !== evidence.test_source_sha256) {
    issues.push('Distinct reviewed baseline commit and matching complete test-source SHA256 for both executions are required.');
  }
  const baseline = Array.isArray(evidence?.baseline_events) ? evidence.baseline_events : [];
  const candidate = Array.isArray(evidence?.candidate_events) ? evidence.candidate_events : [];
  if ([...baseline, ...candidate].some(e => !validDate(e.Time) || Date.parse(e.Time) < expected.started_at)) {
    issues.push('Go event timestamps must belong to the current live observation window, not a replayed baseline log.');
  }
  if (evidence?.baseline_observation_mode !== 'live_test_capture' ||
      evidence?.candidate_observation_mode !== 'live_test_capture' ||
      !validDate(evidence.baseline_measured_at) || !validDate(evidence.candidate_measured_at) ||
      Date.parse(evidence.baseline_measured_at) < expected.started_at ||
      Date.parse(evidence.candidate_measured_at) < expected.started_at) {
    issues.push('Both baseline and candidate must be newly observed in this run; historical/replayed event logs do not count as live red/green proof.');
  }
  const candidateCheck = requiredSubtests(candidate, 'TestMCPFieldFindings', [...required.red, ...required.preserved]);
  issues.push(...candidateCheck.issues);
  for (const name of redNames) {
    const test = `TestMCPFieldFindings/${name}`;
    const events = baseline.filter(e => e.Test === test);
    if (!events.some(e => e.Action === 'run') || !events.some(e => e.Action === 'fail') ||
        events.some(e => e.Action === 'skip')) {
      issues.push(`${test}: baseline must execute and expose the failing behavior with the same assertion, not skip or replay a narrative.`);
    }
  }
  const preserveCheck = requiredSubtests(baseline, 'TestMCPFieldFindings', preservedNames);
  issues.push(...preserveCheck.issues);
  return {
    status: issues.length ? 'blocked' : 'passed', issues,
    reason: issues.length ? 'Live same-test baseline/candidate field-regression evidence is incomplete.'
      : 'Recorded red/green and preservation subtest events match the live-evidence contract; review actual commands, source hashes and fixture provenance independently.',
    baseline_commit: evidence?.baseline_commit ?? null,
    candidate_commit: evidence?.project_commit ?? null,
    scope: 'FE11 is a policy decision, not an invented baseline defect. FE5 must test the lost text branch end-to-end; existing lower-level functionality alone is not a regression fix.'
  };
}
