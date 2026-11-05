import posthog from "posthog-js"

// Production ships without a PostHog key. Callers must still be able to call
// capture() without throwing, and feature-flag code must see analytics as off.
export function disabledPosthog() {
  return {
    __disabled: true,
    capture() {},
    identify() {},
    reset() {},
    get_distinct_id() {
      return undefined
    },
  }
}

export function posthogEnabled(client) {
  return !!(client && !client.__disabled)
}

export default {
  install(Vue) {
    const key = process.env.VUE_APP_POSTHOG_API_KEY
    if (!key) {
      Vue.prototype.$posthog = disabledPosthog()
      return
    }

    Vue.prototype.$posthog = posthog.init(key, {
      api_host: process.env.VUE_APP_POSTHOG_HOST || "https://us.i.posthog.com",
      capture_pageview: false,
      autocapture: false,
    })
  },
}
