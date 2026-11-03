// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from "vitest";
import { parseToolResult } from "../../src/parse";
import { render, type Handlers } from "../../src/render";
import { initialState, reduce, type State } from "../../src/state";
import { answer, answerTooLarge, draft, group, hiddenPeople, pickATime } from "../fixtures";

const handlers = (): Handlers => ({ select: vi.fn(), sort: vi.fn(), view: vi.fn(), tz: vi.fn(), refresh: vi.fn(), loadMore: vi.fn(), expand: vi.fn(), ask: vi.fn(), confirm: vi.fn(), cancelReview: vi.fn() });
const stateFor = (res: unknown): State => reduce(initialState("UTC"), { type: "result", parsed: parseToolResult(res) });
let root: HTMLElement;
beforeEach(() => { document.body.innerHTML = '<div id="app"></div>'; root = document.getElementById("app")!; });

describe("render", () => {
  it("renders names as text, never HTML", () => {
    let s = stateFor(pickATime());
    s = reduce(s, { type: "select", index: 0 });
    render(root, s, handlers());
    expect(root.querySelector("img")).toBeNull();
    expect(root.querySelector("h1 b")).toBeNull();
    expect(root.querySelector("h1")?.textContent).toBe("Q3 <b>Planning</b> & Review");
    expect(root.textContent).toContain("<img src=x onerror=alert(1)>");
  });
  it("labels the reference week and shows candidates in rank order", () => {
    render(root, stateFor(pickATime()), handlers());
    expect(root.textContent).toContain("2018");
    expect(root.textContent).toContain("not dates in the current calendar week");
    const items = [...root.querySelectorAll("button.cand")].map((b) => b.textContent);
    expect(items[0]).toContain("Wed 15:00");
    expect(items[0]).toContain("4 available, 1 if needed, 1 not fully available");
  });
  it("has no write controls or free-text inputs", () => {
    for (const f of [pickATime, answer, group]) {
      render(root, stateFor(f()), handlers());
      const labels = [...root.querySelectorAll("button")].map((b) => b.textContent ?? "");
      expect(labels.join("|")).not.toMatch(/finalize|save|commit|send|remind|delete|remove|add|clear|decline/i);
      expect(root.querySelector("textarea, input[type=text], input:not([type])")).toBeNull();
    }
  });
  it("supports a table alternative to the list", () => {
    let s = stateFor(pickATime());
    s = reduce(s, { type: "view", view: "table" });
    render(root, s, handlers());
    expect(root.querySelectorAll("tbody tr").length).toBe(3);
    expect(root.querySelector("th[scope=col]")).not.toBeNull();
  });
  it("moves focus with arrow keys and selects with click", () => {
    const hd = handlers();
    render(root, stateFor(pickATime()), hd);
    const items = root.querySelectorAll<HTMLElement>("button.cand");
    items[0]!.focus();
    items[0]!.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true, cancelable: true }));
    expect(document.activeElement).toBe(items[1]);
    items[1]!.click();
    expect(hd.select).toHaveBeenCalledWith(1);
  });
  it("shows hidden people without counts", () => {
    render(root, stateFor(hiddenPeople()), handlers());
    expect(root.textContent).toContain("This information is not available to your account");
    expect(root.textContent).not.toMatch(/\d+ participants/);
  });
  it("shows partial group data as incomplete with a Load more control", () => {
    const hd = handlers();
    render(root, stateFor(group()), hd);
    expect(root.textContent).toContain("incomplete");
    expect(root.textContent).toContain("30 participants");
    expect(root.textContent).toContain("Showing 3 on this page");
    const more = [...root.querySelectorAll("button")].find((b) => b.textContent === "Load more")!;
    more.click();
    expect(hd.loadMore).toHaveBeenCalledWith("pc1.3.abc");
  });
  it("answer view shows decline and stored answer separately", () => {
    render(root, stateFor(answer()), handlers());
    expect(root.textContent).toContain("Declined");
    expect(root.textContent).toContain("a stored answer is still on file");
    expect(root.textContent).toContain("2 stored marks are kept");
    expect(root.textContent).toContain("manual availability, 3 manual days");
  });
  it("does not show an incomplete own answer as empty", () => {
    render(root, stateFor(answerTooLarge()), handlers());
    expect(root.textContent).toContain("could not be shown completely");
    expect(root.textContent).not.toContain("None marked");
    expect(root.textContent).not.toContain("cleared");
  });
  it("labels hidden availability with the server reason", () => {
    render(root, stateFor(hiddenPeople()), handlers());
    expect(root.textContent).toContain("hidden availability");
  });
  it("shows a review before confirm", () => {
    const raw = pickATime() as { _meta: { muster: { action_bases: Record<string, string>; permissions: Record<string, boolean> } } };
    raw._meta.muster.action_bases = { write: "fp1" };
    raw._meta.muster.permissions = { ...raw._meta.muster.permissions, can_finalize: true };
    let s = stateFor(raw);
    s = reduce(s, { type: "select", index: 0 });
    const hd = handlers();
    render(root, s, hd);
    expect(root.textContent).toContain("Review this time");
    expect(root.textContent).not.toContain("Nothing is saved until you confirm.");
    root.querySelector<HTMLButtonElement>("[data-fid=review-finalize]")!.click();
    expect(hd.ask).toHaveBeenCalledWith("finalize_time", expect.objectContaining({ start: "2018-06-20T15:00:00Z" }));
    s = reduce(s, { type: "review", actionId: "a1", effect: "Record this time and archive the muster." });
    render(root, s, hd);
    expect(root.textContent).toContain("Nothing is saved until you confirm.");
    root.querySelector<HTMLButtonElement>("[data-fid=confirm]")!.click();
    expect(hd.confirm).toHaveBeenCalledOnce();
  });
  it("renders a draft read-only", () => {
    render(root, stateFor(draft()), handlers());
    expect(root.textContent).toContain("unsaved draft");
    expect(root.textContent).not.toContain("Refresh");
  });
  it("renders loading, error, and fallback states", () => {
    render(root, initialState(), handlers());
    expect(root.querySelector("[aria-busy=true]")).not.toBeNull();
    render(root, reduce(initialState(), { type: "result", parsed: { kind: "error", message: "Nope", code: "X" } }), handlers());
    expect(root.querySelector("[role=alert]")?.textContent).toBe("Nope");
    render(root, reduce(initialState(), { type: "result", parsed: { kind: "fallback", summary: "Just text", text: "" } }), handlers());
    expect(root.textContent).toContain("Just text");
  });
});
