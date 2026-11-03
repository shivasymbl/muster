import { describe, expect, it } from "vitest";
import { initialState, rankedCandidates, reduce } from "../../src/state";
import { parseToolResult } from "../../src/parse";
import { pickATime } from "../fixtures";

const snap = () => parseToolResult(pickATime());

describe("reducer", () => {
  it("starts loading and a result makes it ready", () => {
    const s0 = initialState();
    expect(s0.phase).toBe("loading");
    const s1 = reduce(s0, { type: "result", parsed: snap() });
    expect(s1.phase).toBe("ready");
  });
  it("partial tool input only changes the loading hint", () => {
    const s = reduce(initialState(), { type: "tool_input", workflow: "answer", eventId: "e" });
    expect(s.phase).toBe("loading");
    expect(s.hint.workflow).toBe("answer");
    const ready = reduce(s, { type: "result", parsed: snap() });
    expect(reduce(ready, { type: "tool_input", workflow: "answer", eventId: "x" })).toBe(ready);
  });
  it("ignores obsolete read responses", () => {
    let s = reduce(initialState(), { type: "result", parsed: snap() });
    s = reduce(s, { type: "read_start" });
    const first = s.readSeq;
    s = reduce(s, { type: "read_start" });
    const stale = reduce(s, { type: "result", parsed: { kind: "error", message: "old", code: null }, seq: first });
    expect(stale).toBe(s);
    const fresh = reduce(s, { type: "result", parsed: snap(), seq: s.readSeq });
    expect(fresh.ui.reading).toBe(false);
  });
  it("a host-pushed result supersedes an in-flight read", () => {
    let s = reduce(initialState(), { type: "result", parsed: snap() });
    s = reduce(s, { type: "read_start" });
    const seq = s.readSeq;
    s = reduce(s, { type: "result", parsed: snap() });
    expect(s.ui.reading).toBe(false);
    expect(reduce(s, { type: "result", parsed: { kind: "error", message: "late", code: null }, seq })).toBe(s);
  });
  it("a failed read keeps the validated snapshot and shows a notice", () => {
    let s = reduce(initialState(), { type: "result", parsed: snap() });
    s = reduce(s, { type: "read_start" });
    s = reduce(s, { type: "read_error", message: "Refresh failed.", seq: s.readSeq });
    expect(s.phase).toBe("ready");
    expect(s.ui.notice).toBe("Refresh failed.");
  });
  it("an error result drops the previous snapshot", () => {
    let s = reduce(initialState(), { type: "result", parsed: snap() });
    s = reduce(s, { type: "result", parsed: { kind: "error", message: "x", code: null } });
    expect(s.snapshot).toBeNull();
    expect(s.phase).toBe("error");
  });
  it("teardown clears the snapshot and invalidates reads", () => {
    let s = reduce(initialState(), { type: "result", parsed: snap() });
    s = reduce(s, { type: "read_start" });
    const seq = s.readSeq;
    s = reduce(s, { type: "teardown" });
    expect(s.snapshot).toBeNull();
    expect(s.readSeq).toBeGreaterThan(seq);
  });
  it("cancellation only errors while loading", () => {
    expect(reduce(initialState(), { type: "cancelled" }).phase).toBe("error");
    const ready = reduce(initialState(), { type: "result", parsed: snap() });
    expect(reduce(ready, { type: "cancelled" })).toBe(ready);
  });
  it("sorts by time without reordering the server rank", () => {
    const r = snap();
    if (r.kind !== "snapshot") throw new Error("fixture");
    expect(rankedCandidates(r.snapshot, "rank")).toEqual([0, 1, 2]);
    expect(rankedCandidates(r.snapshot, "time")).toEqual([1, 0, 2]);
  });
});
