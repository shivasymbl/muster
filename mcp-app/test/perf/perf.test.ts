// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { statSync } from "node:fs";
import { parseToolResult } from "../../src/parse";
import { render } from "../../src/render";
import { initialState, reduce } from "../../src/state";
import { manyCandidates } from "../fixtures";

const hd = { select: vi.fn(), sort: vi.fn(), view: vi.fn(), tz: vi.fn(), refresh: vi.fn(), loadMore: vi.fn(), expand: vi.fn(), ask: vi.fn(), confirm: vi.fn(), cancelReview: vi.fn() };

describe("performance budgets", () => {
  it("parses 5000 candidates in under 250 ms", () => {
    const data = manyCandidates(5000);
    const t = performance.now();
    const r = parseToolResult(data);
    const dt = performance.now() - t;
    expect(r.kind).toBe("snapshot");
    expect(dt).toBeLessThan(250);
  });
  it("renders 500 candidates in under 500 ms in both layouts", () => {
    document.body.innerHTML = '<div id="app"></div>';
    const root = document.getElementById("app")!;
    const parsed = parseToolResult(manyCandidates(500));
    let s = reduce(initialState("UTC"), { type: "result", parsed });
    const t = performance.now();
    render(root, s, hd);
    expect(root.querySelectorAll("button.cand").length).toBe(500);
    s = reduce(s, { type: "view", view: "table" });
    render(root, s, hd);
    expect(root.querySelectorAll("tbody tr").length).toBe(500);
    expect(performance.now() - t).toBeLessThan(500);
  });
  it("keeps the bundle small when built", async () => {
    const { buildBundle } = await import("../e2e/mock-host");
    const html = buildBundle();
    expect(Buffer.byteLength(html)).toBeLessThan(1024 * 1024);
  });
});
