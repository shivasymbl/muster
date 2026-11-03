import {
  SUPPORTED_SCHEMA_VERSION, type Candidate, type Candidates, type Counts, type DateMode, type EventInfo, type Grid, type OwnAnswer,
  type ParsedResult, type People, type Permissions, type Person, type Range, type Section, type SectionStatus, type Snapshot, type Workflow,
} from "./types";

const WORKFLOWS: readonly string[] = ["pick-a-time", "answer", "run-a-group"];
const STATUSES: readonly string[] = ["complete", "partial", "hidden", "not_requested", "unavailable"];
const DATE_MODES: readonly string[] = ["actual", "reference_week", "undated", "ambiguous"];
const MAX_ITEMS = 5000;

class ParseError extends Error {}
const fail = (m: string): never => { throw new ParseError(m); };

type Obj = Record<string, unknown>;
const isObj = (v: unknown): v is Obj => typeof v === "object" && v !== null && !Array.isArray(v);
const obj = (v: unknown, p: string): Obj => (isObj(v) ? v : fail(`${p}: expected object`));
const str = (v: unknown, p: string): string => (typeof v === "string" ? v : fail(`${p}: expected string`));
const bool = (v: unknown, p: string): boolean => (typeof v === "boolean" ? v : fail(`${p}: expected boolean`));
const arr = (v: unknown, p: string): unknown[] => {
  if (!Array.isArray(v)) return fail(`${p}: expected array`);
  if (v.length > MAX_ITEMS) return fail(`${p}: too many items`);
  return v;
};
/** Counts: null/absent means hidden/unknown, never zero. Negative or non-integer is rejected, not clamped. */
const count = (v: unknown, p: string): number | null => {
  if (v === undefined || v === null) return null;
  if (typeof v !== "number" || !Number.isInteger(v) || v < 0) return fail(`${p}: invalid count`);
  return v;
};
const iso = (v: unknown, p: string): string => {
  const s = str(v, p);
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:\d{2})$/.test(s) || Number.isNaN(Date.parse(s))) {
    return fail(`${p}: invalid timestamp`);
  }
  return s;
};
const strList = (v: unknown, p: string): string[] => arr(v, p).map((x, i) => str(x, `${p}[${i}]`));
const optStr = (v: unknown, p: string): string | null => (v === undefined || v === null ? null : str(v, p));

function section<T>(raw: unknown, p: string, parse: (d: unknown) => T): Section<T> {
  if (raw === undefined) return { status: "not_requested", reason: null, data: null, next_cursor: null };
  const o = obj(raw, p);
  const status = str(o.status, `${p}.status`);
  if (!STATUSES.includes(status)) fail(`${p}.status: unknown status`);
  const st = status as SectionStatus;
  const reason = optStr(o.reason, `${p}.reason`) || null;
  const cursor = o.next_cursor === undefined || o.next_cursor === null || o.next_cursor === "" ? null : str(o.next_cursor, `${p}.next_cursor`);
  if (st === "hidden" || st === "unavailable" || st === "not_requested") {
    if (o.data !== null && o.data !== undefined) fail(`${p}: ${st} section must not carry data`);
    return { status: st, reason, data: null, next_cursor: cursor };
  }
  if (o.data === null || o.data === undefined) fail(`${p}: ${st} section requires data`);
  return { status: st, reason, data: parse(o.data), next_cursor: cursor };
}

function parseGrid(d: unknown): Grid {
  const o = obj(d, "grid");
  const slot = o.slot_minutes;
  if (typeof slot !== "number" || !Number.isInteger(slot) || slot < 1 || slot > 1440) fail("grid.slot_minutes: invalid");
  const reference_week = o.reference_week === undefined ? false : bool(o.reference_week, "grid.reference_week");
  const days_only = o.days_only === undefined ? false : bool(o.days_only, "grid.days_only");
  const dates = o.dates == null ? [] : strList(o.dates, "grid.dates");
  const m = str(o.date_mode, "grid.date_mode");
  if (!DATE_MODES.includes(m)) fail("grid.date_mode: unknown");
  const date_mode = m as DateMode;
  if (reference_week !== (date_mode === "reference_week")) fail("grid: reference_week conflicts with date_mode");
  return {
    slot_minutes: slot as number, reference_week, days_only, date_mode, dates,
    window_start: o.window_start ? iso(o.window_start, "grid.window_start") : null,
    window_end: o.window_end ? iso(o.window_end, "grid.window_end") : null,
  };
}

function reqCount(v: unknown, p: string): number {
  const c = count(v, p);
  return c === null ? fail(`${p}: required count missing`) : c;
}

function parseCandidates(d: unknown): Candidates {
  const o = obj(d, "candidates");
  const items = arr(o.items, "candidates.items").map((raw, i): Candidate => {
    const p = `candidates.items[${i}]`;
    const c = obj(raw, p);
    const start = iso(c.start, `${p}.start`);
    const end = iso(c.end, `${p}.end`);
    if (Date.parse(end) <= Date.parse(start)) fail(`${p}: end before start`);
    if (c.population !== "stored_respondents") fail(`${p}.population: expected stored_respondents`);
    return {
      start, end, label: optStr(c.label, `${p}.label`),
      duration_minutes: count(c.duration_minutes, `${p}.duration_minutes`),
      available: reqCount(c.available, `${p}.available`),
      if_needed: reqCount(c.if_needed, `${p}.if_needed`),
      not_fully_available: reqCount(c.not_fully_available, `${p}.not_fully_available`),
      not_fully_available_labels: c.not_fully_available_labels == null ? [] : strList(c.not_fully_available_labels, `${p}.not_fully_available_labels`),
      population: "stored_respondents",
    };
  });
  return { duration_minutes: count(o.duration_minutes, "candidates.duration_minutes"), complete: bool(o.complete, "candidates.complete"), items };
}

function parseRanges(v: unknown, p: string): Range[] {
  if (v == null) return [];
  return arr(v, p).map((r, i) => {
    const o = obj(r, `${p}[${i}]`);
    const start = iso(o.start, `${p}[${i}].start`);
    const end = iso(o.end, `${p}[${i}].end`);
    if (Date.parse(end) <= Date.parse(start)) fail(`${p}[${i}]: end before start`);
    return { start, end };
  });
}

function parsePerson(raw: unknown, p: string): Person {
  const o = obj(raw, p);
  return {
    key: str(o.key, `${p}.key`), name: str(o.name ?? "", `${p}.name`), email: optStr(o.email, `${p}.email`) || null,
    status: str(o.status, `${p}.status`), is_me: o.is_me === undefined ? false : bool(o.is_me, `${p}.is_me`),
    available: parseRanges(o.available, `${p}.available`), if_needed: parseRanges(o.if_needed, `${p}.if_needed`),
  };
}

function parsePeople(d: unknown): People {
  const o = obj(d, "people");
  const items = o.items == null ? [] : arr(o.items, "people.items").map((x, i) => parsePerson(x, `people.items[${i}]`));
  if (new Set(items.map((x) => x.key)).size !== items.length) fail("people.items: duplicate keys");
  const totals: Record<string, number> = {};
  if (o.totals != null) for (const [k, v] of Object.entries(obj(o.totals, "people.totals"))) totals[k] = reqCount(v, `people.totals.${k}`);
  return { complete: bool(o.complete, "people.complete"), total: reqCount(o.total, "people.total"), totals, items };
}

function parseCounts(v: unknown, p: string): Counts | null {
  if (v == null) return null;
  const o = obj(v, p);
  return { in_window: reqCount(o.in_window, `${p}.in_window`), off_window: reqCount(o.off_window, `${p}.off_window`), off_grid: reqCount(o.off_grid, `${p}.off_grid`) };
}

function parseOwn(d: unknown): OwnAnswer {
  const o = obj(d, "own_answer");
  const marks = (v: unknown, p: string): string[] => (v == null ? [] : arr(v, p).map((x, i) => iso(x, `${p}[${i}]`)));
  return {
    exists: bool(o.exists, "own_answer.exists"),
    my_status: str(o.my_status, "own_answer.my_status"),
    is_attendee: o.is_attendee === undefined ? false : bool(o.is_attendee, "own_answer.is_attendee"),
    declined: o.declined === undefined ? false : bool(o.declined, "own_answer.declined"),
    available: marks(o.available, "own_answer.available"),
    if_needed: marks(o.if_needed, "own_answer.if_needed"),
    editable_layers: o.editable_layers == null ? [] : strList(o.editable_layers, "own_answer.editable_layers"),
    preserved_layers: o.preserved_noneditable_layers == null ? [] : strList(o.preserved_noneditable_layers, "own_answer.preserved_noneditable_layers"),
    manual_days: count(o.manual_days, "own_answer.manual_days") ?? 0,
    available_counts: parseCounts(o.available_counts, "own_answer.available_counts"),
    if_needed_counts: parseCounts(o.if_needed_counts, "own_answer.if_needed_counts"),
    editable: o.editable === undefined ? false : bool(o.editable, "own_answer.editable"),
    complete: bool(o.complete, "own_answer.complete"),
  };
}

function parsePermissions(raw: unknown): Permissions {
  const o = obj(raw, "permissions");
  const b = (k: string): boolean => (o[k] === undefined ? false : bool(o[k], `permissions.${k}`));
  return {
    can_rank: b("can_rank"), can_view_people: b("can_view_people"), can_edit_own_answer: b("can_edit_own_answer"),
    can_manage_group: b("can_manage_group"), can_finalize: b("can_finalize"),
  };
}

function parseEvent(raw: unknown): EventInfo {
  const o = obj(raw, "event");
  let scheduled: EventInfo["scheduled"] = null;
  if (o.scheduled !== null && o.scheduled !== undefined) {
    const s = obj(o.scheduled, "event.scheduled");
    scheduled = { start: iso(s.start, "event.scheduled.start"), end: iso(s.end, "event.scheduled.end") };
    if (Date.parse(scheduled.end) <= Date.parse(scheduled.start)) fail("event.scheduled: end before start");
  }
  return {
    id: str(o.id, "event.id"), name: str(o.name, "event.name"), kind: typeof o.kind === "string" ? o.kind : "event",
    archived: o.archived === undefined ? false : bool(o.archived, "event.archived"), scheduled,
    scheduled_status: typeof o.scheduled_status === "string" ? o.scheduled_status : scheduled ? "set" : "none",
    blind_availability: o.blind_availability === undefined ? false : bool(o.blind_availability, "event.blind_availability"),
  };
}

export function parseSnapshot(meta: unknown, summary: string): Snapshot {
  const m = obj(meta, "_meta.muster");
  if (m.schema_version !== SUPPORTED_SCHEMA_VERSION) fail(`unsupported schema_version ${String(m.schema_version)}`);
  const workflow = str(m.workflow, "workflow");
  if (!WORKFLOWS.includes(workflow)) fail("workflow: unknown");
  const draft = m.draft === undefined ? false : bool(m.draft, "draft");
  const s = obj(m.sections ?? {}, "sections");
  if (!draft && (m.event === null || m.event === undefined)) fail("event: required");
  return {
    workflow: workflow as Workflow, draft,
    snapshot_id: optStr(m.snapshot_id, "snapshot_id"),
    read_at: m.read_at == null ? null : iso(m.read_at, "read_at"),
    summary,
    event: m.event == null ? null : parseEvent(m.event),
    permissions: parsePermissions(m.permissions ?? {}),
    grid: section(s.grid, "grid", parseGrid),
    candidates: section(s.candidates, "candidates", parseCandidates),
    people: section(s.people, "people", parsePeople),
    own_answer: section(s.own_answer, "own_answer", parseOwn),
  };
}

interface ToolResultLike { content?: unknown; structuredContent?: unknown; _meta?: unknown; isError?: unknown }

function firstText(content: unknown): string {
  if (!Array.isArray(content)) return "";
  for (const c of content) if (isObj(c) && c.type === "text" && typeof c.text === "string") return c.text;
  return "";
}

/** Validate a tool result. Never throws; never trusts content[0] as a snapshot. */
export function parseToolResult(result: unknown): ParsedResult {
  if (!isObj(result)) return { kind: "error", message: "The host returned an unreadable result.", code: null };
  const r = result as ToolResultLike;
  const text = firstText(r.content);
  if (r.isError === true) {
    let message = text || "Muster could not open this view.";
    let code: string | null = null;
    try {
      const j: unknown = JSON.parse(text);
      if (isObj(j)) {
        const d = isObj(j.data) ? j.data : {};
        if (typeof j.message === "string") message = j.message;
        else if (typeof d.message === "string") message = d.message;
        const c = typeof d.code === "string" ? d.code : typeof j.code === "string" ? j.code : null;
        code = c;
      }
    } catch { /* plain-message error */ }
    return { kind: "error", message, code };
  }
  const sc = isObj(r.structuredContent) ? r.structuredContent : {};
  const summary = typeof sc.summary === "string" ? sc.summary : text;
  const meta = isObj(r._meta) ? r._meta.muster : undefined;
  if (meta === undefined || meta === null) return { kind: "fallback", summary, text };
  // The server may send a metadata stub with no event/sections yet: that is a text fallback, not a broken view.
  if (isObj(meta) && meta.event === undefined && meta.sections === undefined) return { kind: "fallback", summary, text };
  try {
    const snap = parseSnapshot(meta, summary);
    // A snapshot with no usable sections still renders; the renderer labels each section.
    return { kind: "snapshot", snapshot: snap };
  } catch (e) {
    const why = e instanceof ParseError ? e.message : "invalid snapshot";
    return { kind: "error", message: `This view could not be shown (${why}). Use the text summary or open Muster.`, code: "INVALID_SNAPSHOT" };
  }
}
