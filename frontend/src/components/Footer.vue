<template>
  <footer class="tw-border-t tw-border-light-gray-stroke tw-bg-white tw-px-6 tw-py-10 tw-text-ink">
    <div
      class="tw-mx-auto tw-flex tw-max-w-6xl tw-flex-col tw-gap-8 md:tw-flex-row md:tw-items-start md:tw-justify-between"
    >
      <div>
        <Logo color="navy" :width="140" />
        <div class="tw-mt-3 tw-flex tw-items-center tw-gap-3 tw-text-sm tw-text-slate">
          <span>An Asymbl product</span>
          <img
            src="@/assets/brand/asymbl-logo-navy.png"
            alt="Asymbl"
            class="tw-h-5 tw-w-auto"
          />
        </div>
      </div>
      <nav class="tw-flex tw-flex-col tw-gap-2 tw-text-sm">
        <router-link :to="{ name: 'privacy-policy' }" class="tw-text-ink">
          Privacy
        </router-link>
        <a href="https://github.com/shivasymbl/muster" class="tw-text-ink">
          Source code
        </a>
        <a href="mailto:muster@asymbl.com" class="tw-text-ink">Contact</a>
      </nav>
    </div>
    <p class="tw-mx-auto tw-mt-8 tw-max-w-6xl tw-text-xs tw-text-slate">
      © 2026 Asymbl Inc. Muster is open source under AGPL-3.0 and built on
      Timeful by the Schej team.
    </p>
  </footer>
</template>

<script>
import Logo from "@/components/Logo.vue"

export default {
  name: "Footer",
  components: { Logo },
}
</script>
