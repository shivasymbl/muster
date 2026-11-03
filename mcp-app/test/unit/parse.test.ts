import { describe, expect, it } from "vitest";
import { parseToolResult } from "../../src/parse";
import { answer, draft, envelope, errorResult, group, hiddenPeople, pickATime } from "../fixtures";

const clone = <T,>(x: T): T => JSON.parse(JSON.stringify(x)) as T;

describe("parseToolResult", () => {
  it("parses the three workflows", () => {
    for (const [f, wf] of [[pickATime, "pick-a-time"], [answer, "answer"], [group, "run-a-group"]] as const) {
      const r = parseToolResult(f());
      expect(r.kind).toBe("snapshot");
      if (r.kind === "snapshot") expect(r.snapshot.workflow).toBe(wf);
    }
  });
  it("keeps hidden sections hidden with no data", () => {
    const r = parseToolResult(hiddenPeople());
    expect(r.kind === "snapshot" && r.snapshot.people.status).toBe("hidden");
    expect(r.kind === "snapshot" && r.snapshot.people.data).toBeNull();
  });
  it("rejects a hidden section that carries data", () => {
    const x = clone(hiddenPeople()) as any;
    x._meta.muster.sections.people.data = { complete: true, total: 0, totals: {}, items: [] };
    expect(parseToolResult(x).kind).toBe("error");
  });
  it("rejects unknown schema versions instead of guessing", () => {
    const x = clone(pickATime()) as any;
    x._meta.muster.schema_version = 2;
    const r = parseToolResult(x);
    expect(r.kind).toBe("error");
  });
  it("rejects negative counts rather than clamping", () => {
    const x = clone(pickATime()) as any;
    x._meta.muster.sections.candidates.data.items[0].available = -1;
    expect(parseToolResult(x).kind).toBe("error");
  });
  it("rejects a candidate with a missing count instead of showing zero", () => {
    const x = clone(pickATime()) as any;
    delete x._meta.muster.sections.candidates.data.items[0].available;
    expect(parseToolResult(x).kind).toBe("error");
  });
  it("parses an unsaved draft with no event", () => {
    const r = parseToolResult(draft());
    expect(r.kind === "snapshot" && r.snapshot.draft).toBe(true);
    expect(r.kind === "snapshot" && r.snapshot.event).toBeNull();
  });
  it("requires an event unless the snapshot is a draft", () => {
    const x = clone(pickATime()) as any;
    x._meta.muster.event = null;
    expect(parseToolResult(x).kind).toBe("error");
  });
  it("keeps section reasons", () => {
    const r = parseToolResult(hiddenPeople());
    expect(r.kind === "snapshot" && r.snapshot.people.reason).toBe("hidden_availability");
  });
  it("rejects reversed candidate intervals and bad timestamps", () => {
    const x = clone(pickATime()) as any;
    x._meta.muster.sections.candidates.data.items[0].end = "2018-06-20T14:00:00Z";
    expect(parseToolResult(x).kind).toBe("error");
    const y = clone(pickATime()) as any;
    y._meta.muster.sections.candidates.data.items[0].start = "tomorrow";
    expect(parseToolResult(y).kind).toBe("error");
  });
  it("rejects duplicate participant keys but allows duplicate names", () => {
    const x = clone(group()) as any;
    expect(parseToolResult(x).kind).toBe("snapshot");
    x._meta.muster.sections.people.data.items[2].key = "p2";
    expect(parseToolResult(x).kind).toBe("error");
  });
  it("falls back to the summary when _meta.muster is absent, never parsing content text", () => {
    const r = parseToolResult({ content: [{ type: "text", text: '{"schema_version":1,"workflow":"answer"}' }] });
    expect(r.kind).toBe("fallback");
  });
  it("falls back for a metadata stub with no event or sections", () => {
    const r = parseToolResult(envelope("answer", "No snapshot yet.", { snapshot: null }));
    expect(r.kind).toBe("fallback");
  });
  it("reads structured tool errors", () => {
    const r = parseToolResult(errorResult());
    expect(r).toEqual({ kind: "error", message: "Event not found.", code: "NOT_FOUND" });
  });
  it("handles plain-text errors and junk without throwing", () => {
    expect(parseToolResult({ isError: true, content: [{ type: "text", text: "boom" }] })).toEqual({ kind: "error", message: "boom", code: null });
    for (const junk of [null, 5, "x", [], {}, { content: 7 }]) expect(() => parseToolResult(junk)).not.toThrow();
  });
  it("flags reference_week/date_mode conflicts", () => {
    const x = clone(pickATime()) as any;
    x._meta.muster.sections.grid.data.date_mode = "actual";
    expect(parseToolResult(x).kind).toBe("error");
  });
});
