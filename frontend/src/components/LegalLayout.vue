<template>
  <div class="tw-min-h-screen tw-bg-white tw-text-ink">
    <header
      class="tw-sticky tw-top-0 tw-z-20 tw-flex tw-items-center tw-justify-between tw-gap-4 tw-border-b tw-border-light-gray-stroke tw-bg-white tw-px-6 tw-py-3"
    >
      <router-link :to="{ name: 'landing' }" class="tw-leading-none">
        <Logo color="navy" :width="132" />
      </router-link>
      <nav class="tw-flex tw-gap-4 tw-text-sm tw-font-medium">
        <router-link class="tw-text-ink" :to="{ name: 'landing' }">Home</router-link>
        <router-link class="tw-text-ink" :to="{ name: 'privacy-policy' }">Privacy</router-link>
        <router-link class="tw-text-ink" :to="{ name: 'terms' }">Terms</router-link>
      </nav>
    </header>
    <div
      class="tw-px-6 tw-py-14 tw-text-white"
      style="background: linear-gradient(180deg, #191d47, #2c1169)"
    >
      <div class="tw-mx-auto tw-max-w-3xl">
        <p class="tw-text-xs tw-font-semibold tw-uppercase tw-tracking-wide" style="color: #b2deff">
          Asymbl Muster
        </p>
        <h1 class="tw-mt-2 tw-text-4xl tw-font-semibold">{{ title }}</h1>
      </div>
    </div>
    <main class="tw-mx-auto tw-max-w-3xl tw-px-6 tw-py-12 tw-leading-relaxed">
      <p
        class="tw-mb-8 tw-inline-block tw-rounded-full tw-bg-ligher-green tw-px-3 tw-py-2 tw-text-sm tw-text-slate"
      >
        This page needs review by Asymbl legal before it is treated as final.
      </p>
      <slot />
    </main>
    <Footer />
  </div>
</template>

<script>
import Footer from "@/components/Footer.vue"
import Logo from "@/components/Logo.vue"

export default {
  name: "LegalLayout",
  components: { Footer, Logo },
  props: {
    title: { type: String, required: true },
  },
}
</script>
