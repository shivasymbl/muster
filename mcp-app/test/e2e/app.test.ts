import { beforeAll, describe, expect, it } from "vitest";
import { answer, errorResult, group, hiddenPeople, pickATime } from "../fixtures";
import { buildBundle, mockHost, tick, type MockHost } from "./mock-host";

let html: string;
beforeAll(() => { html = buildBundle(); });

async function open(args: Record<string, unknown>, result?: unknown): Promise<MockHost> {
  const h = mockHost(html);
  await h.ready;
  h.notify("ui/notifications/tool-input", { arguments: args });
  if (result !== undefined) h.notify("ui/notifications/tool-result", result);
  await tick();
  return h;
}
const doc = (h: MockHost): Document => h.dom.window.document;
const text = (h: MockHost): string => doc(h).body.textContent ?? "";

describe("built bundle", () => {
  it("is a self-contained HTML document within limits", () => {
    expect(html.startsWith("<!doctype html>")).toBe(true);
    expect(Buffer.byteLength(html)).toBeLessThanOrEqual(2 * 1024 * 1024);
    expect(html).not.toMatch(/https?:\/\//i);
    expect(html).not.toMatch(/<script[^>]*\ssrc\s*=|<link\b|@import|\s(src|href)\s*=\s*["']?(https?:)?\/\//i);
  });
  it("has narrow-screen CSS with no fixed pixel widths over 320", () => {
    expect(html).toContain("@media (max-width: 360px)");
    for (const m of html.matchAll(/(?:^|[;{\s])(?:min-)?width:\s*(\d+)px/g)) expect(Number(m[1])).toBeLessThanOrEqual(320);
    expect(html).toContain('name="viewport"');
  });
  it("contains no token, cookie or fetch/XHR path of its own", () => {
    expect(html).not.toMatch(/Authorization|document\.cookie|localStorage|sessionStorage|XMLHttpRequest|new WebSocket|EventSource\(/);
  });
});

describe("mock host flows", () => {
  it("connects, shows loading, then renders pick-a-time", async () => {
    const h = mockHost(html);
    await h.ready;
    expect(text(h)).toContain("Loading");
    h.notify("ui/notifications/tool-input-partial", { arguments: { workflow: "pick-a-time" } });
    await tick();
    expect(text(h)).toContain("Pick a time");
    h.notify("ui/notifications/tool-result", pickATime());
    await tick();
    expect(doc(h).querySelector("h1")?.textContent).toBe("Q3 <b>Planning</b> & Review");
    expect(doc(h).querySelectorAll("button.cand").length).toBe(3);
    expect(text(h)).toContain("2018");
    expect(h.calls.length).toBe(0);
  });
  it("applies host theme variables and safe areas", async () => {
    const h = await open({ workflow: "pick-a-time", event_id: "ev-1" }, pickATime());
    const root = doc(h).documentElement;
    expect(root.style.getPropertyValue("--color-background-primary")).toBe("#101010");
    expect(root.style.getPropertyValue("--safe-bottom")).toBe("3px");
  });
  it("sends no credentials in any message to the host", async () => {
    const h = await open({ workflow: "pick-a-time", event_id: "ev-1" }, pickATime());
    expect(JSON.stringify(h.received)).not.toMatch(/bearer|authorization|token/i);
  });
  it("renders answer and group workflows", async () => {
    const a = await open({ workflow: "answer", event_id: "ev-2" }, answer());
    expect(text(a)).toContain("Declined");
    expect(a.dom.window.document.getElementById("tz")).not.toBeNull();
    const g = await open({ workflow: "run-a-group", event_id: "ev-3" }, group());
    expect(text(g)).toContain("30 participants");
  });
  it("shows hidden state and error state", async () => {
    const hid = await open({ workflow: "run-a-group", event_id: "ev-3" }, hiddenPeople());
    expect(text(hid)).toContain("not available to your account");
    const err = await open({ workflow: "answer", event_id: "nope" }, errorResult());
    expect(doc(err).querySelector("[role=alert]")?.textContent).toBe("Event not found.");
  });
  it("refresh goes through callServerTool with the same read-only arguments", async () => {
    const h = await open({ workflow: "pick-a-time", event_id: "ev-1", duration_minutes: 60 }, pickATime());
    h.toolHandler = () => pickATime();
    (doc(h).querySelector('[data-fid="refresh"]') as HTMLElement).click();
    await tick(50);
    expect(h.calls).toEqual([{ name: "muster_open_view", arguments: { workflow: "pick-a-time", event_id: "ev-1", duration_minutes: 60 } }]);
  });
  it("load more passes the cursor and a failed read keeps the view with a notice", async () => {
    const h = await open({ workflow: "run-a-group", event_id: "ev-3" }, group());
    h.toolHandler = () => { throw new Error("upstream down"); };
    const more = [...doc(h).querySelectorAll("button")].find((b) => b.textContent === "Load more")!;
    more.click();
    await tick(50);
    expect(h.calls[0]!.arguments.people_cursor).toBe("pc1.3.abc");
    expect(doc(h).querySelector("[role=alert]")?.textContent).toContain("Refresh failed");
    expect(text(h)).toContain("Book club");
  });
  it("only the newest overlapping read is applied", async () => {
    const h = await open({ workflow: "pick-a-time", event_id: "ev-1" }, pickATime());
    let release!: () => void;
    h.toolHandler = () => new Promise((r) => { release = () => r(answer()); });
    (doc(h).querySelector('[data-fid="refresh"]') as HTMLElement).click();
    await tick();
    // A newer tool-result from the host arrives while the read is in flight.
    h.notify("ui/notifications/tool-result", group());
    await tick();
    release();
    await tick(50);
    expect(doc(h).querySelector("h1")?.textContent).toBe("Book club");
  });
  it("handles display-mode refusal without breaking the view", async () => {
    const h = await open({ workflow: "pick-a-time", event_id: "ev-1" }, pickATime());
    h.displayModeResponse = "refuse";
    (doc(h).querySelector('[data-fid="expand"]') as HTMLElement).click();
    await tick(50);
    expect(doc(h).querySelector("[role=alert]")?.textContent).toContain("did not allow a larger view");
    expect(doc(h).querySelectorAll("button.cand").length).toBe(3);
  });
  it("teardown clears the snapshot and stops reads", async () => {
    const h = await open({ workflow: "pick-a-time", event_id: "ev-1" }, pickATime());
    const res = await h.request("ui/resource-teardown", {});
    expect(res.error).toBeUndefined();
    expect(doc(h).querySelector("#app")?.childNodes.length).toBe(0);
    h.notify("ui/notifications/tool-result", pickATime());
    await tick();
    expect(doc(h).querySelector("#app")?.childNodes.length).toBe(0);
  });
  it("cancellation before data shows an error state", async () => {
    const h = mockHost(html);
    await h.ready;
    h.notify("ui/notifications/tool-cancelled", { reason: "user" });
    await tick();
    expect(text(h)).toContain("cancelled");
  });
  it("makes no background calls while idle", async () => {
    const h = await open({ workflow: "pick-a-time", event_id: "ev-1" }, pickATime());
    const before = h.received.length;
    await tick(300);
    expect(h.calls.length).toBe(0);
    expect(h.received.filter((m) => m.method === "tools/call").length).toBe(0);
    expect(h.received.length - before).toBeLessThanOrEqual(2); // size-change notifications only
  });
});
