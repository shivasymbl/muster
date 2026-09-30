<template>
  <div class="tw-min-h-screen tw-bg-white tw-text-ink">
    <header
      class="tw-sticky tw-top-0 tw-z-20 tw-flex tw-items-center tw-justify-between tw-gap-4 tw-border-b tw-border-light-gray-stroke tw-bg-white/95 tw-px-6 tw-py-3"
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
    <main class="tw-mx-auto tw-max-w-3xl tw-px-6 tw-py-14">
      <p class="tw-text-xs tw-font-semibold tw-uppercase tw-tracking-wide tw-text-green">
        Asymbl Muster
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
}
</script>
