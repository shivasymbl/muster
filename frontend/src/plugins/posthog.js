import posthog from "posthog-js"

export default {
  install(Vue) {
    const key = process.env.VUE_APP_POSTHOG_API_KEY
    if (!key) {
      Vue.prototype.$posthog = null
      return
    }

    Vue.prototype.$posthog = posthog.init(key, {
      api_host: process.env.VUE_APP_POSTHOG_HOST || "https://us.i.posthog.com",
      capture_pageview: false,
      autocapture: false,
    })
  },
}
