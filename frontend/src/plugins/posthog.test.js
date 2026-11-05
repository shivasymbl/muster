import { readFileSync } from "fs"
import { describe, expect, it } from "vitest"
import { disabledPosthog, posthogEnabled } from "./posthog"

describe("PostHog when no key is configured", () => {
  it("does not throw when a click records an event", () => {
    const client = disabledPosthog()
    expect(() =>
      client.capture("create_new_button_clicked", { eventOnly: false })
    ).not.toThrow()
    expect(posthogEnabled(client)).toBe(false)
    expect(posthogEnabled(null)).toBe(false)
  })

  it("runs the create action even if capture would throw", () => {
    const calls = []
    const createNew = (payload) => calls.push(payload)
    const posthog = {
      capture() {
        throw new TypeError("Cannot read properties of null (reading 'capture')")
      },
    }
    function createNewClick(eventOnly = false) {
      createNew({ eventOnly })
      posthog?.capture("create_new_button_clicked", { eventOnly })
    }
    expect(() => createNewClick(false)).toThrow()
    expect(calls).toEqual([{ eventOnly: false }])
  })

  it("keeps each calendar day at least 8.5rem wide", () => {
    const source = readFileSync(
      new URL("../components/schedule_overlap/ScheduleOverlap.vue", import.meta.url),
      "utf8"
    )
    const mins = source.match(/tw-min-w-\[8\.5rem\]/g) || []
    expect(mins.length).toBeGreaterThanOrEqual(2)
    expect(source).toContain("tw-overflow-x-auto")
  })
})
