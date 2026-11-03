import { formatRange, mergeSlots, minutesLabel, zoneLabel } from "./format";
import { rankedCandidates, type State } from "./state";
import type { Section, Snapshot } from "./types";

export interface Handlers {
  select(index: number | null): void;
  sort(s: "rank" | "time"): void;
  view(v: "list" | "table"): void;
  tz(tz: string): void;
  refresh(): void;
  loadMore(cursor: string): void;
  expand(): void;
  ask(operation: string, payload: Record<string, unknown>): void;
  confirm(): void;
  cancelReview(): void;
}

type Child = Node | string | null | undefined | false;
/** Element builder. Text is always added as a text node; there is no HTML injection path. */
export function h(tag: string, attrs: Record<string, string | boolean | undefined> = {}, ...kids: Child[]): HTMLElement {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === undefined || v === false) continue;
    el.setAttribute(k, v === true ? "" : v);
  }
  for (const k of kids) if (k) el.append(typeof k === "string" ? document.createTextNode(k) : k);
  return el;
}

const WORKFLOW_TITLE = { "pick-a-time": "Pick a time", answer: "Answer availability", "run-a-group": "Run a group" } as const;

const REASONS: Record<string, string> = {
  sign_up_form: "This is a sign-up form, so this section does not apply.",
  hidden_availability: "The organizer has hidden availability for this event.",
  undated: "This event has no dates, so there are no candidate times.",
  candidate_cap: "Only the first 20 candidates are listed.",
  page: "Showing one page of participants.",
  own_answer_too_large: "Your stored answer is too large to show safely here.",
  duplicate_own_response: "More than one stored answer exists for you, so editing is turned off. The rest of this muster is still shown.",
};
const reasonText = (r: string | null): string => (r ? REASONS[r] ?? "" : "");

const STATUS_LABEL: Record<string, string> = {
  responded: "Answered", no_answer: "No answer yet", invited_no_answer: "Invited, no answer yet", invited: "Invited, no answer yet",
  cleared: "Answer cleared", declined: "Declined",
};
const statusLabel = (s: string): string => STATUS_LABEL[s] ?? s;

function btn(label: string, on: () => void, opts: { pressed?: boolean; disabled?: boolean; id?: string } = {}): HTMLElement {
  const b = h("button", { type: "button", "data-fid": opts.id, "aria-pressed": opts.pressed === undefined ? undefined : String(opts.pressed), disabled: opts.disabled }, label);
  b.addEventListener("click", on);
  return b;
}

function notAvailable(what: string, sec: Section<unknown>): HTMLElement {
  const why = reasonText(sec.reason);
  if (sec.status === "hidden") return h("p", { class: "state hidden", role: "note" }, `${what}: This information is not available to your account.${why ? " " + why : ""}`);
  if (sec.status === "not_requested") return h("p", { class: "state incomplete", role: "note" }, `${what}: not loaded in this view.`);
  return h("p", { class: "state incomplete", role: "note" }, `${what}: unavailable. ${why || "Refresh to try again."}`);
}

function incompleteBanner(what: string, sec: Section<unknown>, hd: Handlers): HTMLElement | null {
  if (sec.status !== "partial") return null;
  return h("div", { class: "state incomplete", role: "note" },
    `${what} is incomplete. ${reasonText(sec.reason) || "Counts below cover only what was loaded."} `,
    sec.next_cursor ? btn("Load more", () => hd.loadMore(sec.next_cursor as string), { id: "more-" + what }) : null);
}

function header(snap: Snapshot, state: State): HTMLElement {
  const ev = snap.event;
  const mode = snap.grid.data?.date_mode ?? "actual";
  return h("header", {},
    h("p", { class: "kicker" }, WORKFLOW_TITLE[snap.workflow]),
    h("h1", {}, ev ? ev.name : "New group draft"),
    h("p", { class: "meta" }, snap.action_base ? "A change is saved only after you review it and confirm." : "Read-only view. Nothing here changes your schedule."),
    ev?.archived ? h("p", { class: "badge" }, "Archived") : null,
    ev?.archived ? h("p", { class: "meta" }, "Archived does not lock your answer.") : null,
    ev?.scheduled ? h("p", { class: "meta" }, `Stored final time: ${formatRange(ev.scheduled.start, ev.scheduled.end, mode, state.ui.tz)}`) : null,
    ev && ev.scheduled_status === "invalid" ? h("p", { class: "state incomplete", role: "note" }, "The stored final time could not be read.") : null);
}

function toolbar(state: State, snap: Snapshot, hd: Handlers): HTMLElement {
  const grid = snap.grid.data;
  const kids: Child[] = [];
  if (grid && grid.date_mode === "actual") {
    const zones = [...new Set(["UTC", Intl.DateTimeFormat().resolvedOptions().timeZone, state.ui.tz])].filter(Boolean);
    const sel = h("select", { id: "tz", "data-fid": "tz" }, ...zones.map((z) => h("option", { value: z, selected: z === state.ui.tz }, z)));
    (sel as HTMLSelectElement).value = state.ui.tz;
    sel.addEventListener("change", () => hd.tz((sel as HTMLSelectElement).value));
    kids.push(h("label", { for: "tz" }, "Show times in ", sel));
  }
  if (!snap.draft) kids.push(btn(state.ui.reading ? "Refreshing\u2026" : "Refresh", hd.refresh, { disabled: state.ui.reading, id: "refresh" }));
  if (state.ui.canExpand && state.ui.displayMode !== "fullscreen") kids.push(btn("Expand", hd.expand, { id: "expand" }));
  return h("div", { class: "toolbar", role: "group", "aria-label": "View controls" }, ...kids);
}

function pickATime(state: State, snap: Snapshot, hd: Handlers): Node[] {
  const out: Node[] = [];
  const sec = snap.candidates;
  if (sec.status !== "complete" && sec.status !== "partial") return [notAvailable("Candidate times", sec)];
  const banner = incompleteBanner("Candidate times", sec, hd);
  if (banner) out.push(banner);
  const data = sec.data!;
  const list = data.items;
  const mode = snap.grid.data?.date_mode ?? "actual";
  const tz = state.ui.tz;
  if (data.duration_minutes) out.push(h("p", { class: "meta" }, `Requested duration: ${minutesLabel(data.duration_minutes)}.`));
  out.push(h("p", { class: "meta" }, "Counts cover stored respondents only. Not-fully-available people may be conditional or unavailable, so the groups do not add up to a total."));
  if (list.length === 0) {
    out.push(h("p", { class: "state empty" }, sec.status === "complete" ? "No candidate time fits in this window." : "No candidates loaded yet."));
    return out;
  }
  const order = rankedCandidates(snap, state.ui.sort);
  const best = list[0]!;
  out.push(h("p", { class: "best" }, best.not_fully_available !== 0
    ? "No full fit: every candidate leaves someone not fully available."
    : "Best candidate fits every stored respondent."));
  out.push(h("div", { class: "toolbar", role: "group", "aria-label": "Sort and layout" },
    btn("Best first", () => hd.sort("rank"), { pressed: state.ui.sort === "rank", id: "sort-rank" }),
    btn("By time", () => hd.sort("time"), { pressed: state.ui.sort === "time", id: "sort-time" }),
    btn("List", () => hd.view("list"), { pressed: state.ui.view === "list", id: "view-list" }),
    btn("Table", () => hd.view("table"), { pressed: state.ui.view === "table", id: "view-table" })));

  if (state.ui.view === "table") {
    const tbl = h("table", {},
      h("caption", {}, "Candidate times"),
      h("thead", {}, h("tr", {}, ...["Time", "Available", "If needed", "Not fully available"].map((t) => h("th", { scope: "col" }, t)))),
      h("tbody", {}, ...order.map((i) => {
        const c = list[i]!;
        return h("tr", {}, h("th", { scope: "row" }, formatRange(c.start, c.end, mode, tz)),
          h("td", {}, String(c.available)), h("td", {}, String(c.if_needed)), h("td", {}, String(c.not_fully_available)));
      })));
    out.push(h("div", { class: "tablewrap", tabindex: "0", role: "region", "aria-label": "Candidate times table" }, tbl));
    return out;
  }

  const ol = h("ol", { class: "cands", "aria-label": "Candidate times, best first unless sorted by time" });
  order.forEach((i, pos) => {
    const c = list[i]!;
    const sel = state.ui.selected === i;
    const b = h("button", { type: "button", class: "cand", "data-fid": "cand-" + i, "aria-pressed": String(sel) },
      h("strong", {}, formatRange(c.start, c.end, mode, tz)),
      h("span", { class: "counts" }, `${c.available} available, ${c.if_needed} if needed, ${c.not_fully_available} not fully available`));
    b.addEventListener("click", () => hd.select(sel ? null : i));
    b.addEventListener("keydown", (e) => {
      const ev = e as KeyboardEvent;
      const step = ev.key === "ArrowDown" || ev.key === "ArrowRight" ? 1 : ev.key === "ArrowUp" || ev.key === "ArrowLeft" ? -1 : 0;
      if (ev.key === "Home" || ev.key === "End" || step) {
        ev.preventDefault();
        const items = ol.querySelectorAll<HTMLElement>("button.cand");
        const next = ev.key === "Home" ? 0 : ev.key === "End" ? items.length - 1 : Math.min(items.length - 1, Math.max(0, pos + step));
        items[next]?.focus();
      }
    });
    const li = h("li", {}, b);
    if (sel) {
      li.append(h("div", { class: "detail", role: "region", "aria-label": "Selected time details" },
        h("p", {}, `Selected locally: ${formatRange(c.start, c.end, mode, tz)}.`),
        h("p", {}, c.not_fully_available_labels.length
          ? `Not fully available: ${c.not_fully_available_labels.join(", ")}`
          : c.not_fully_available === 0 ? "Everyone stored is fully available." : "Names are not shown."),
        h("p", { class: "meta" }, snap.action_base && snap.permissions.can_finalize
          ? "Review records this exact interval. It does not create a calendar event and it sends no email."
          : "Selecting a time does not record it."),
        snap.action_base && snap.permissions.can_finalize
          ? btn("Review this time", () => hd.ask("finalize_time", { start: c.start, end: c.end }), { id: "review-finalize" })
          : null));
    }
    ol.append(li);
  });
  out.push(ol);
  return out;
}

function answer(state: State, snap: Snapshot, hd: Handlers): Node[] {
  const sec = snap.own_answer;
  if (sec.status !== "complete" && sec.status !== "partial") return [notAvailable("Your answer", sec)];
  const a = sec.data!;
  const out: Node[] = [];
  const banner = incompleteBanner("Your answer", sec, hd);
  if (banner) out.push(banner);
  if (!a.complete || sec.status === "partial") out.push(h("p", { class: "state incomplete", role: "note" }, "Your stored answer could not be shown completely, so it is not shown at all. Open Muster to see it."));
  const mode = snap.grid.data?.date_mode ?? "actual";
  const slot = snap.grid.data?.slot_minutes ?? 15;
  const outside = (a.available_counts?.off_window ?? 0) + (a.if_needed_counts?.off_window ?? 0);
  const total = a.available.length + a.if_needed.length;
  out.push(h("dl", { class: "facts" },
    h("dt", {}, "Your status"), h("dd", {}, statusLabel(a.my_status)),
    h("dt", {}, "Answer on file"), h("dd", {}, !a.exists ? "No answer recorded" : total || !a.complete ? "Yes" : "Yes, cleared (no times marked)"),
    h("dt", {}, "Invitation"), h("dd", {}, a.declined ? "Declined" : a.is_attendee ? "Invited" : "Not on the invite list"),
    a.preserved_layers.length || a.manual_days ? h("dt", {}, "Kept, not shown") : null,
    a.preserved_layers.length || a.manual_days
      ? h("dd", {}, [...a.preserved_layers.map((l) => l.replace(/_/g, " ")), a.manual_days ? `${a.manual_days} manual days` : ""].filter(Boolean).join(", "))
      : null,
    a.complete && outside > 0 ? h("dt", {}, "Outside this window") : null,
    a.complete && outside > 0 ? h("dd", {}, `${outside} stored marks are kept and listed below.`) : null));
  if (a.declined && a.exists) out.push(h("p", { class: "meta" }, "You declined this invitation, but a stored answer is still on file."));
  if (a.complete) {
    const groups: [string, string[]][] = [["Available", a.available], ["Available if needed", a.if_needed]];
    for (const [label, marks] of groups) {
      const ranges = mergeSlots(marks, slot);
      out.push(h("h2", {}, `${label} (${ranges.length} ${ranges.length === 1 ? "range" : "ranges"})`));
      out.push(ranges.length
        ? h("ul", { class: "ranges", "aria-label": label }, ...ranges.map((r) => h("li", {}, formatRange(r.start, r.end, mode, state.ui.tz))))
        : h("p", { class: "meta" }, "None marked."));
    }
  }
  if (snap.action_base && snap.permissions.can_edit_own_answer && a.complete && a.editable) {
    out.push(btn("Review clearing my answer", () => hd.ask("clear_own_answer", {}), { id: "review-clear" }));
    out.push(h("p", { class: "meta" }, "Clearing removes availability and if-needed marks. It does not delete manual calendar marks."));
  } else {
    out.push(h("p", { class: "meta" }, a.editable ? "Editing is not available in this view." : "This answer cannot be edited here."));
  }
  return out;
}

function group(state: State, snap: Snapshot, hd: Handlers): Node[] {
  const sec = snap.people;
  if (sec.status !== "complete" && sec.status !== "partial") return [notAvailable("Participants", sec)];
  const p = sec.data!;
  const out: Node[] = [];
  const banner = incompleteBanner("Participants", sec, hd);
  if (banner) out.push(banner);
  const partial = sec.status === "partial" || !p.complete;
  const breakdown = Object.entries(p.totals).filter(([, n]) => n > 0).map(([k, n]) => `${n} ${statusLabel(k).toLowerCase()}`).join(", ");
  out.push(h("p", { class: "summary-line" }, `${p.total} participants${breakdown ? ": " + breakdown : ""}.${partial ? ` Showing ${p.items.length} on this page.` : ""}`));
  if (p.items.length === 0 && !partial) out.push(h("p", { class: "state empty" }, "No participants yet."));
  else out.push(h("ul", { class: "people", "aria-label": "Participants" }, ...p.items.map((x) =>
    h("li", {}, h("span", { class: "name" }, x.name || "(no name)"), x.is_me ? h("span", { class: "badge" }, "You") : null,
      h("span", { class: "status" }, statusLabel(x.status)),
      x.available.length + x.if_needed.length ? h("span", { class: "meta" }, `${x.available.length} available, ${x.if_needed.length} if needed ranges`) : null,
      x.email ? h("span", { class: "email" }, x.email) : null,
      snap.action_base && snap.permissions.can_manage_group && x.email && !x.is_me
        ? btn("Review remove", () => hd.ask("remove_attendee", { email: x.email }), { id: "review-remove" })
        : null))));
  if (snap.action_base && snap.permissions.can_manage_group) {
    const input = h("input", { type: "email", id: "invite-email", "data-fid": "invite-email", placeholder: "name@example.com" });
    out.push(h("div", { class: "toolbar" }, input, btn("Review invite", () => hd.ask("add_attendees", { emails: [(input as HTMLInputElement).value] }), { id: "review-invite" })));
  }
  const me = snap.own_answer.data;
  if (snap.action_base && me?.is_attendee && !me.declined) {
    out.push(btn("Review declining my invitation", () => hd.ask("decline_own_invitation", {}), { id: "review-decline" }));
  }
  if (!snap.action_base) out.push(h("p", { class: "meta" }, "Group changes are not available in this view."));
  void state;
  return out;
}

function ready(state: State, snap: Snapshot, hd: Handlers): Node[] {
  const out: Node[] = [header(snap, state)];
  if (snap.summary) out.push(h("p", { class: "summary" }, snap.summary));
  if (snap.draft) {
    out.push(h("p", { class: "state empty" }, "This is an unsaved draft definition. Nothing is created from this view. Ask Claude to continue, or open Muster."));
    return out;
  }
  const gridSec = snap.grid;
  if (gridSec.status === "hidden" || gridSec.status === "unavailable") out.push(notAvailable("Schedule grid", gridSec));
  else out.push(h("p", { class: "meta tzlabel" }, zoneLabel(gridSec.data, state.ui.tz)));
  if (gridSec.data?.date_mode === "ambiguous") out.push(h("p", { class: "state incomplete", role: "note" }, "This event mixes date types, so it is read-only and shown in UTC."));
  out.push(toolbar(state, snap, hd));
  if (state.ui.notice) out.push(h("p", { class: "state error", role: "alert" }, state.ui.notice));
  if (state.ui.review) {
    out.push(h("div", { class: "detail", role: "region", "aria-label": "Review" },
      h("p", {}, "Review"),
      h("p", {}, state.ui.review.effect),
      h("p", { class: "meta" }, "Nothing is saved until you confirm."),
      btn("Confirm", hd.confirm, { id: "confirm", disabled: state.ui.reading }),
      btn("Cancel", hd.cancelReview, { id: "cancel-review" })));
  }
  if (snap.workflow === "pick-a-time") out.push(...pickATime(state, snap, hd));
  else if (snap.workflow === "answer") out.push(...answer(state, snap, hd));
  else out.push(...group(state, snap, hd));
  return out;
}

/** Render state into root. Focus is restored by data-fid after the rebuild. */
export function render(root: HTMLElement, state: State, hd: Handlers): void {
  const active = document.activeElement as HTMLElement | null;
  const fid = active?.getAttribute?.("data-fid") ?? null;
  const kids: Node[] = [];
  switch (state.phase) {
    case "loading":
      kids.push(h("div", { class: "skeleton", role: "status", "aria-live": "polite", "aria-busy": "true" },
        h("p", { class: "kicker" }, state.hint.workflow ? WORKFLOW_TITLE[state.hint.workflow] : "Muster"),
        h("p", {}, "Loading…"), h("div", { class: "bar" }), h("div", { class: "bar short" })));
      break;
    case "fallback":
      kids.push(h("h1", {}, "Muster"), h("p", { class: "summary" }, state.fallback?.summary || "No interactive view is available."),
        h("p", { class: "meta" }, "The interactive view has no data for this result. Read the summary above, or refresh."),
        btn(state.ui.reading ? "Refreshing…" : "Refresh", hd.refresh, { id: "refresh", disabled: state.ui.reading }));
      break;
    case "error":
      kids.push(h("h1", {}, "Muster"), h("p", { class: "state error", role: "alert" }, state.error?.message ?? "Something went wrong."));
      if (state.error?.code) kids.push(h("p", { class: "meta" }, `Code: ${state.error.code}`));
      kids.push(btn("Try again", hd.refresh, { id: "refresh", disabled: state.ui.reading }));
      break;
    case "ready":
      kids.push(...ready(state, state.snapshot!, hd));
      break;
  }
  root.replaceChildren(...kids);
  if (fid) root.querySelector<HTMLElement>(`[data-fid="${CSS.escape(fid)}"]`)?.focus();
}
