import type { ParsedResult, Snapshot, Workflow } from "./types";

export type Phase = "loading" | "ready" | "fallback" | "error";
export interface UiState {
  selected: number | null;
  sort: "rank" | "time";
  view: "list" | "table";
  tz: string;
  displayMode: string;
  canExpand: boolean;
  reading: boolean;
  notice: string | null;
  review: { actionId: string; effect: string } | null;
}
export interface State {
  phase: Phase;
  /** Nonsensitive context from tool input while loading. */
  hint: { workflow: Workflow | null; eventId: string | null };
  snapshot: Snapshot | null;
  fallback: { summary: string; text: string } | null;
  error: { message: string; code: string | null } | null;
  ui: UiState;
  /** Monotonic id of the newest read; older responses are ignored. */
  readSeq: number;
}

export type Action =
  | { type: "tool_input"; workflow: Workflow | null; eventId: string | null }
  | { type: "result"; parsed: ParsedResult; seq?: number }
  | { type: "read_start" }
  | { type: "read_error"; message: string; seq: number }
  | { type: "cancelled" }
  | { type: "select"; index: number | null }
  | { type: "sort"; sort: "rank" | "time" }
  | { type: "view"; view: "list" | "table" }
  | { type: "tz"; tz: string }
  | { type: "host"; displayMode?: string; canExpand?: boolean; tz?: string }
  | { type: "notice"; message: string | null }
  | { type: "review"; actionId: string; effect: string }
  | { type: "clear_review" }
  | { type: "teardown" };

// Muster stores no timezone, so the default display zone is UTC; the user may pick their browser zone.
export function initialState(tz = "UTC"): State {
  return {
    phase: "loading", hint: { workflow: null, eventId: null }, snapshot: null, fallback: null, error: null, readSeq: 0,
    ui: { selected: null, sort: "rank", view: "list", tz, displayMode: "inline", canExpand: false, reading: false, notice: null, review: null },
  };
}

export function reduce(s: State, a: Action): State {
  switch (a.type) {
    case "tool_input":
      // Partial or complete input only labels the skeleton. It never replaces a validated snapshot.
      return s.phase === "loading" ? { ...s, hint: { workflow: a.workflow, eventId: a.eventId } } : s;
    case "read_start":
      return { ...s, readSeq: s.readSeq + 1, ui: { ...s.ui, reading: true, notice: null } };
    case "result": {
      // A numbered result from an obsolete read is ignored.
      if (a.seq !== undefined && a.seq !== s.readSeq) return s;
      // A host-pushed result (no seq) supersedes any read still in flight.
      if (a.seq === undefined) s = { ...s, readSeq: s.readSeq + 1 };
      const ui = { ...s.ui, reading: false, selected: null, review: null };
      const p = a.parsed;
      if (p.kind === "snapshot") {
        return { ...s, phase: "ready", snapshot: p.snapshot, fallback: null, error: null, ui };
      }
      if (p.kind === "fallback") return { ...s, phase: "fallback", snapshot: null, fallback: { summary: p.summary, text: p.text }, error: null, ui };
      // On error keep no stale sensitive snapshot: the failed read leaves the view unverified.
      return { ...s, phase: "error", snapshot: null, fallback: null, error: { message: p.message, code: p.code }, ui };
    }
    case "read_error":
      if (a.seq !== s.readSeq) return s;
      return { ...s, ui: { ...s.ui, reading: false, notice: a.message } };
    case "cancelled":
      return s.phase === "loading"
        ? { ...s, phase: "error", error: { message: "The request was cancelled before the view loaded.", code: "CANCELLED" } }
        : s;
    case "select": return { ...s, ui: { ...s.ui, selected: a.index } };
    case "sort": return { ...s, ui: { ...s.ui, sort: a.sort, selected: null } };
    case "view": return { ...s, ui: { ...s.ui, view: a.view } };
    case "tz": return { ...s, ui: { ...s.ui, tz: a.tz } };
    case "host":
      return { ...s, ui: {
        ...s.ui, displayMode: a.displayMode ?? s.ui.displayMode, canExpand: a.canExpand ?? s.ui.canExpand,
        tz: s.snapshot ? s.ui.tz : a.tz ?? s.ui.tz,
      } };
    case "notice": return { ...s, ui: { ...s.ui, notice: a.message } };
    case "review": return { ...s, ui: { ...s.ui, reading: false, review: { actionId: a.actionId, effect: a.effect }, notice: null } };
    case "clear_review": return { ...s, ui: { ...s.ui, review: null } };
    case "teardown":
      // Clear sensitive data; any in-flight read becomes obsolete.
      return { ...initialState(s.ui.tz), readSeq: s.readSeq + 1 };
  }
}

export function rankedCandidates(s: Snapshot, sort: "rank" | "time"): number[] {
  const list = s.candidates.data?.items ?? [];
  const idx = list.map((_, i) => i);
  if (sort === "time") return idx.sort((a, b) => Date.parse(list[a]!.start) - Date.parse(list[b]!.start));
  // Server order is the rank. "rank" keeps it.
  return idx;
}
