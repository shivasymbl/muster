import { describe, expect, it } from "vitest";
import { REFERENCE_WEEK_LABEL, formatRange, mergeSlots, zoneLabel } from "../../src/format";

describe("format", () => {
  it("labels reference-week times as UTC weekday and clock", () => {
    expect(formatRange("2018-06-20T15:00:00Z", "2018-06-20T16:00:00Z", "reference_week", "America/Los_Angeles")).toBe("Wed 15:00–16:00 UTC");
    expect(formatRange("2018-06-17T09:00:00Z", "2018-06-17T09:30:00Z", "reference_week", "UTC")).toBe("Sun 09:00–09:30 UTC");
  });
  it("explains the 2018 reference week, not the current week", () => {
    expect(REFERENCE_WEEK_LABEL).toContain("2018");
    expect(REFERENCE_WEEK_LABEL).toContain("UTC");
    expect(REFERENCE_WEEK_LABEL).toMatch(/not dates in the current calendar week/);
    expect(zoneLabel({ slot_minutes: 30, reference_week: true, days_only: false, dates: [], date_mode: "reference_week", window_start: null, window_end: null }, "Asia/Tokyo")).toBe(REFERENCE_WEEK_LABEL);
  });
  it("converts actual instants to the chosen zone and names it", () => {
    expect(formatRange("2030-11-04T15:00:00Z", "2030-11-04T15:30:00Z", "actual", "America/New_York")).toContain("10:00–10:30 America/New_York");
    expect(formatRange("2030-11-04T15:00:00Z", "2030-11-04T15:30:00Z", "actual", "Not/AZone")).toContain("UTC");
  });
  it("merges adjacent slots into half-open ranges", () => {
    const r = mergeSlots(["2030-11-04T15:15:00Z", "2030-11-04T15:00:00Z", "2030-11-04T16:00:00Z", "2030-11-04T15:00:00Z"], 15);
    expect(r).toEqual([
      { start: "2030-11-04T15:00:00.000Z", end: "2030-11-04T15:30:00.000Z", slots: 2 },
      { start: "2030-11-04T16:00:00.000Z", end: "2030-11-04T16:15:00.000Z", slots: 1 },
    ]);
    expect(mergeSlots([], 15)).toEqual([]);
  });
});
