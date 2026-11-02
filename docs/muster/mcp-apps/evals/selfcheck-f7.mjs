#!/usr/bin/env node
// Negative harness-parser checks only. These synthetic rejection fixtures are
// never application tests, live traces, metrics, owner approval or release evidence.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { requiredSubtests, ownerDecisions, textEfficiency, fieldRegression, catalogStage } from './f7-evidence.mjs';
const here = path.dirname(fileURLToPath(import.meta.url));
const read = name => JSON.parse(fs.readFileSync(path.join(here, name), 'utf8'));
const manifest = read('eval-manifest.json');
const expected = { run_id: 'negative-parser-check', project_commit: 'a'.repeat(40), started_at: Date.now(), release_stage: 'text-only' };

const parentOnly = [
  { Package: 'negative-fixture', Test: 'TestMCPFieldFindings', Action: 'run' },
  { Package: 'negative-fixture', Test: 'TestMCPFieldFindings', Action: 'pass' }
];
assert.equal(requiredSubtests(parentOnly, 'TestMCPFieldFindings', manifest.required_subtests.TestMCPFieldFindings).subtests.length, 21);
assert.equal(requiredSubtests(parentOnly, 'TestMCPFieldFindings', manifest.required_subtests.TestMCPFieldFindings).issues.length, 21);
const nestedSkip = [
  ...parentOnly,
  { Package: 'negative-fixture', Test: 'TestMCPFieldFindings/FE1', Action: 'run' },
  { Package: 'negative-fixture', Test: 'TestMCPFieldFindings/FE1', Action: 'pass' },
  { Package: 'negative-fixture', Test: 'TestMCPFieldFindings/FE1/assertion', Action: 'skip' }
];
assert.equal(requiredSubtests(nestedSkip, 'TestMCPFieldFindings', ['FE1']).subtests[0].status, 'blocked');
assert.equal(ownerDecisions(read('owner_decisions.template.json'), manifest.owner_decisions).status, 'blocked');
assert.equal(ownerDecisions(null, manifest.owner_decisions).decisions.filter(x => x.status === 'pending').length, 6);
assert.equal(textEfficiency(read('text-efficiency.template.json'), expected, manifest.text_efficiency).status, 'blocked');
assert.equal(fieldRegression(read('field-regression.template.json'), expected, manifest.field_regression).status, 'blocked');

const efficiency = read('text-efficiency.template.json');
efficiency.cases.find(c => c.id === 'availability_known').records = [1, 2].map(id => ({
  origin: 'model',
  request_raw: JSON.stringify({ jsonrpc: '2.0', id, method: id === 1 ? 'resources/read' : 'tools/call', params: {} }),
  response_raw: JSON.stringify({ jsonrpc: '2.0', id, error: { code: -1, message: 'Deliberately rejected synthetic parser fixture' } })
}));
efficiency.setup_records = [{
  request_raw: JSON.stringify({ jsonrpc: '2.0', id: 3, method: 'resources/read', params: {} }),
  response_raw: JSON.stringify({ jsonrpc: '2.0', id: 3, error: { code: -1, message: 'Negative fixture' } })
}];
const rejected = textEfficiency(efficiency, expected, manifest.text_efficiency);
assert.equal(rejected.status, 'blocked');
assert.equal(rejected.measured.cases.find(c => c.id === 'availability_known').model_mcp_calls, 2);
assert(rejected.issues.some(x => x.includes('model MCP-call budget exceeded')));
assert(rejected.issues.some(x => x.includes('cannot be relabeled as excluded setup')));
// Structural catalog fixtures only, not captures or passing product metrics.
const base = manifest.text_efficiency.base_tool_names.map(name => ({ name }));
const opener = { name: manifest.text_efficiency.opener_tool };
const helpers = manifest.text_efficiency.app_only_helpers.map(name => ({ name, description: 'App only structural parser fixture' }));
assert.equal(catalogStage(base, 'text-only', manifest.text_efficiency).issues.length, 0);
assert.equal(catalogStage([...base, opener], 'text-only', manifest.text_efficiency).issues.length, 0);
assert(catalogStage(base, 'full-build', manifest.text_efficiency).issues.length > 0);
assert(catalogStage([...base, opener], 'full-build', manifest.text_efficiency).issues.length > 0);
assert(catalogStage([...base, opener, helpers[0]], 'text-only', manifest.text_efficiency).issues.length > 0);
assert(catalogStage([...base, opener, ...helpers.slice(0, 2)], 'text-only', manifest.text_efficiency).issues.length > 0);
assert.equal(catalogStage([...base, opener, ...helpers], 'full-build', manifest.text_efficiency).issues.length, 0);
const falselyDeclaredStage = { ...read('text-efficiency.template.json'), release_stage: 'text-only' };
assert(textEfficiency(falselyDeclaredStage, { ...expected, release_stage: 'full-build' }, manifest.text_efficiency).issues
  .some(x => x.includes('cannot self-declare text-only')));
console.log('F7 negative harness checks passed: parent-only green, nested skips, pending policies, null/stale evidence and hidden/retried calls cannot pass. No application tests or live measurements were executed.');
