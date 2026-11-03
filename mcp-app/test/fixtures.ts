// Deterministic fixtures in the live server shape (server/services/mcp/snapshot.go).
// Reference-week times use the 2018 reference week (Mon 2018-06-18 ... Sun 2018-06-24, UTC).
export type Json = Record<string, any>;

const perms = { can_rank: true, can_view_people: true, can_edit_own_answer: false, can_manage_group: false, can_finalize: false };
const sec = (status: string, data: unknown = null, reason?: string, next_cursor?: string): Json =>
  ({ status, ...(reason ? { reason } : {}), data, ...(next_cursor ? { next_cursor } : {}) });
const NR = (): Json => sec("not_requested");

export function envelope(workflow: string, summary: string, muster: Json | null, extra: Json = {}): Json {
  return {
    content: [{ type: "text", text: summary }],
    structuredContent: { schema_version: 1, workflow, summary, read_only: true },
    ...(muster ? { _meta: { muster: { schema_version: 1, snapshot_id: "snap-1", state_fingerprint: "fp", action_bases: {}, read_at: "2030-11-04T10:00:00Z", workflow, draft: false, ...muster } } } : {}),
    ...extra,
  };
}

const refGrid = (): Json => sec("complete", { slot_minutes: 30, days_only: false, start_on_monday: true, reference_week: true, date_mode: "reference_week", dates: [], window_start: "2018-06-18T00:00:00Z", window_end: "2018-06-25T00:00:00Z" });
const ev = (o: Json = {}): Json => ({ id: "ev-1", name: "Q3 <b>Planning</b> & Review", kind: "dow", archived: false, scheduled: null, scheduled_status: "none", blind_availability: false, is_sign_up_form: false, ...o });
const cand = (start: string, end: string, available: number, if_needed: number, nfa: number, who: string[]): Json =>
  ({ start, end, duration_minutes: 60, available, if_needed, not_fully_available: nfa, not_fully_available_labels: who, population: "stored_respondents" });

export const pickATime = (): Json => envelope("pick-a-time", "Q3 Planning: 3 candidate times; best has 4 available and 1 if needed.", {
  event: ev(), permissions: perms,
  sections: {
    grid: refGrid(),
    candidates: sec("complete", { duration_minutes: 60, complete: true, items: [
      cand("2018-06-20T15:00:00Z", "2018-06-20T16:00:00Z", 4, 1, 1, ["Ana <img src=x onerror=alert(1)>"]),
      cand("2018-06-19T09:00:00Z", "2018-06-19T10:00:00Z", 3, 2, 2, []),
      cand("2018-06-22T13:30:00Z", "2018-06-22T14:30:00Z", 3, 0, 2, []),
    ] }),
    people: NR(), own_answer: NR(),
  },
});

export const answer = (): Json => envelope("answer", "Offsite: no candidate times to show. Your status: declined.", {
  event: ev({ id: "ev-2", name: "Offsite", kind: "specific_dates" }),
  permissions: { ...perms, can_rank: false, can_edit_own_answer: true },
  sections: {
    grid: sec("complete", { slot_minutes: 15, days_only: false, start_on_monday: false, reference_week: false, date_mode: "actual", dates: ["2030-11-04T00:00:00Z"], window_start: "2030-11-04T00:00:00Z", window_end: "2030-11-11T00:00:00Z" }),
    candidates: NR(), people: NR(),
    own_answer: sec("complete", {
      exists: true, my_status: "declined", is_attendee: true, declined: true,
      available: ["2030-11-04T15:00:00Z", "2030-11-04T15:15:00Z", "2030-11-04T16:00:00Z"], if_needed: ["2030-11-04T17:00:00Z"],
      editable_layers: ["available", "if_needed"], preserved_noneditable_layers: ["manual_availability"], manual_days: 3,
      available_counts: { in_window: 3, off_window: 2, off_grid: 0 }, if_needed_counts: { in_window: 1, off_window: 0, off_grid: 0 },
      editable: true, complete: true, target_fingerprint: "t",
    }),
  },
});

export const answerTooLarge = (): Json => {
  const a = answer();
  a._meta.muster.sections.own_answer = sec("partial", { exists: true, my_status: "responded", is_attendee: true, declined: false, available: [], if_needed: [], editable_layers: [], preserved_noneditable_layers: [], manual_days: 0, available_counts: null, if_needed_counts: null, editable: false, complete: false }, "own_answer_too_large");
  return a;
};

const person = (key: string, name: string, status: string, extra: Json = {}): Json => ({ key, name, status, is_me: false, available: [], if_needed: [], ...extra });

export const group = (): Json => envelope("run-a-group", "Book club: no candidate times to show.", {
  event: ev({ id: "ev-3", name: "Book club", kind: "group", scheduled: { start: "2018-06-21T18:00:00Z", end: "2018-06-21T19:00:00Z" }, scheduled_status: "set" }),
  permissions: { ...perms, can_manage_group: true },
  sections: {
    grid: refGrid(), candidates: NR(),
    people: sec("partial", { complete: false, total: 30, totals: { responded: 1, no_answer: 1, declined: 1 }, items: [
      person("p1", "Owner One", "responded", { is_me: true, email: "owner@example.com", available: [{ start: "2018-06-20T15:00:00Z", end: "2018-06-20T16:00:00Z" }] }),
      person("p2", "Sam", "no_answer"),
      person("p3", "Sam", "declined"),
    ] }, "page", "pc1.3.abc"),
    own_answer: NR(),
  },
});

export const hiddenPeople = (): Json => envelope("run-a-group", "Book club.", {
  event: ev({ id: "ev-3", name: "Book club", kind: "group", blind_availability: true }),
  permissions: { ...perms, can_rank: false, can_view_people: false },
  sections: { grid: refGrid(), candidates: sec("hidden", null, "hidden_availability"), people: sec("hidden", null, "hidden_availability"), own_answer: NR() },
});

export const draft = (): Json => envelope("run-a-group", "New group draft. Nothing is saved until the group is created.", {
  draft: true, event: null, permissions: { ...perms, can_rank: false, can_view_people: false },
  sections: { grid: sec("complete", { slot_minutes: 15, days_only: false, start_on_monday: false, reference_week: false, date_mode: "undated", dates: [] }), candidates: NR(), people: NR(), own_answer: NR() },
});

export const errorResult = (): Json => ({
  isError: true,
  content: [{ type: "text", text: JSON.stringify({ code: "NOT_FOUND", message: "Event not found.", data: { code: "NOT_FOUND", message: "Event not found." } }) }],
});

export function manyCandidates(n: number): Json {
  const base = pickATime();
  const t0 = Date.parse("2018-06-18T00:00:00Z");
  base._meta.muster.sections.candidates.data.items = Array.from({ length: n }, (_, i) =>
    cand(new Date(t0 + i * 1800000).toISOString(), new Date(t0 + i * 1800000 + 3600000).toISOString(), i % 7, i % 3, i % 5, []));
  return base;
}
