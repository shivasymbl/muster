export const SUPPORTED_SCHEMA_VERSION = 1;
export type Workflow = "pick-a-time" | "answer" | "run-a-group";
export type SectionStatus = "complete" | "partial" | "hidden" | "not_requested" | "unavailable";
export type DateMode = "actual" | "reference_week" | "undated" | "ambiguous";

export interface Section<T> {
  status: SectionStatus;
  /** Why a section is hidden, unavailable, or partial (server reason code). */
  reason: string | null;
  data: T | null;
  next_cursor: string | null;
}
export interface Grid {
  slot_minutes: number;
  reference_week: boolean;
  days_only: boolean;
  date_mode: DateMode;
  dates: string[];
  window_start: string | null;
  window_end: string | null;
}
export interface Candidate {
  start: string;
  end: string;
  label: string | null;
  duration_minutes: number | null;
  available: number;
  if_needed: number;
  not_fully_available: number;
  not_fully_available_labels: string[];
  population: "stored_respondents";
}
export interface Candidates {
  duration_minutes: number | null;
  complete: boolean;
  items: Candidate[];
}
export interface Range { start: string; end: string }
export interface Person {
  key: string;
  name: string;
  email: string | null;
  status: string;
  is_me: boolean;
  available: Range[];
  if_needed: Range[];
}
export interface People {
  complete: boolean;
  total: number;
  totals: Record<string, number>;
  items: Person[];
}
export interface Counts { in_window: number; off_window: number; off_grid: number }
export interface OwnAnswer {
  exists: boolean;
  my_status: string;
  is_attendee: boolean;
  declined: boolean;
  /** Slot-start instants (RFC 3339) of the whole stored answer. */
  available: string[];
  if_needed: string[];
  editable_layers: string[];
  preserved_layers: string[];
  manual_days: number;
  available_counts: Counts | null;
  if_needed_counts: Counts | null;
  editable: boolean;
  complete: boolean;
}
export interface Permissions {
  can_rank: boolean;
  can_view_people: boolean;
  can_edit_own_answer: boolean;
  can_manage_group: boolean;
  can_finalize: boolean;
}
export interface EventInfo {
  id: string;
  name: string;
  kind: string;
  archived: boolean;
  scheduled: { start: string; end: string } | null;
  scheduled_status: string;
  blind_availability: boolean;
}
export interface Snapshot {
  workflow: Workflow;
  draft: boolean;
  snapshot_id: string | null;
  read_at: string | null;
  summary: string;
  /** Server comparison value for a confirmed write. Absent means this view cannot save. */
  action_base: string | null;
  event: EventInfo | null;
  permissions: Permissions;
  grid: Section<Grid>;
  candidates: Section<Candidates>;
  people: Section<People>;
  own_answer: Section<OwnAnswer>;
}

export type ParsedResult =
  | { kind: "snapshot"; snapshot: Snapshot }
  | { kind: "fallback"; summary: string; text: string }
  | { kind: "error"; message: string; code: string | null };
