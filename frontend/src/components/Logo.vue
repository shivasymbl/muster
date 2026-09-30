<template>
  <img
    :alt="alt"
    class="tw-h-auto tw-shrink-0 tw-cursor-pointer"
    :src="src"
    :style="{ width: widthCss }"
  />
</template>

<script>
export default {
  name: "Logo",
  inheritAttrs: false,

  props: {
    color: {
      type: String,
      default: "navy",
      validator: (value) => value === "navy" || value === "white",
    },
    width: {
      type: [Number, String],
      default: 120,
    },
  },

  computed: {
    alt() {
      return "Asymbl Muster"
    },
    src() {
      if (this.color === "white") {
        return require("@/assets/brand/muster-wordmark-white.png")
      }
      return require("@/assets/brand/muster-wordmark-navy.png")
    },
    widthCss() {
      return typeof this.width === "number" ? `${this.width}px` : this.width
    },
  },
}
</script>
