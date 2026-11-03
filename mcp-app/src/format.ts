import type { DateMode, Grid } from "./types";

export const REFERENCE_WEEK_LABEL =
  "Weekly reference week: Mon 18 June to Sun 24 June 2018, in UTC. These are weekday and clock times, not dates in the current calendar week.";

const DOW = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const pad = (n: number): string => String(n).padStart(2, "0");

export function isValidTimeZone(tz: string): boolean {
  try { new Intl.DateTimeFormat("en-US", { timeZone: tz }); return true; } catch { return false; }
}

function utcClock(d: Date): string { return `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`; }

/** Format [start,end) for display. Reference-week grids always use the UTC weekday and clock. */
export function formatRange(start: string, end: string, mode: DateMode, tz: string): string {
  const s = new Date(start);
  const e = new Date(end);
  if (mode === "reference_week") {
    const sameDay = s.getUTCDay() === e.getUTCDay() && e.getTime() - s.getTime() < 86400000;
    return `${DOW[s.getUTCDay()]} ${utcClock(s)}–${sameDay ? "" : DOW[e.getUTCDay()] + " "}${utcClock(e)} UTC`;
  }
  const zone = isValidTimeZone(tz) ? tz : "UTC";
  const day = new Intl.DateTimeFormat("en-US", { timeZone: zone, weekday: "short", month: "short", day: "numeric" });
  const clock = new Intl.DateTimeFormat("en-US", { timeZone: zone, hour: "2-digit", minute: "2-digit", hour12: false });
  const sd = day.format(s);
  const ed = day.format(e);
  const label = zone === "UTC" ? "UTC" : zone;
  return `${sd} ${clock.format(s)}–${sd === ed ? "" : ed + " "}${clock.format(e)} ${label}`;
}

export function zoneLabel(grid: Grid | null, tz: string): string {
  if (grid?.date_mode === "reference_week") return REFERENCE_WEEK_LABEL;
  if (grid?.date_mode === "undated") return "This group has no dates. Times are not tied to a calendar.";
  if (grid?.date_mode === "ambiguous") return "Date type could not be determined; shown read-only in UTC.";
  return `Times shown in ${tz === "UTC" ? "UTC" : tz}.`;
}

/** Merge slot starts into half-open ranges. Marks are slot starts of `slotMinutes` each. */
export function mergeSlots(starts: string[], slotMinutes: number): { start: string; end: string; slots: number }[] {
  const ms = slotMinutes * 60000;
  const t = [...new Set(starts.map((s) => Date.parse(s)))].sort((a, b) => a - b);
  const out: { start: string; end: string; slots: number }[] = [];
  let cur: { a: number; b: number; n: number } | null = null;
  for (const x of t) {
    if (cur && x === cur.b) { cur.b = x + ms; cur.n++; continue; }
    if (cur) out.push({ start: new Date(cur.a).toISOString(), end: new Date(cur.b).toISOString(), slots: cur.n });
    cur = { a: x, b: x + ms, n: 1 };
  }
  if (cur) out.push({ start: new Date(cur.a).toISOString(), end: new Date(cur.b).toISOString(), slots: cur.n });
  return out;
}

export function minutesLabel(m: number): string {
  if (m < 60) return `${m} min`;
  const h = Math.floor(m / 60);
  const r = m % 60;
  return r ? `${h} h ${r} min` : `${h} h`;
}
