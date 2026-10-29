import Vue from "vue"
import Vuetify from "vuetify/lib"
import tailwind from "../../tailwind.config"

Vue.use(Vuetify)

export default new Vuetify({
  theme: {
    themes: {
      light: {
        primary: tailwind.theme.colors.green,
        secondary: "#8856FF",
        accent: "#ED489E",
        error: "#ED2929",
        success: "#00CC6B",
        warning: "#FFBB00",
      },
    },
  },
  breakpoint: {
    thresholds: {
      xs: 640,
      sm: 768,
      md: 1024,
      lg: 1280,
    },
    scrollBarWidth: 0,
  },
})
