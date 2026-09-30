<template>
  <LegalLayout>
    <h1 class="tw-mt-2 tw-text-4xl tw-font-semibold">Privacy policy</h1>
    <p
      class="tw-mt-4 tw-inline-block tw-rounded-full tw-bg-ligher-green tw-px-3 tw-py-2 tw-text-sm tw-text-slate"
    >
      This page needs review by Asymbl legal before it is treated as final.
    </p>
    <div class="tw-mt-8 tw-space-y-4 tw-leading-relaxed">
      <p>
        Asymbl Inc is the controller for Asymbl Muster. Questions go to
        <a class="tw-text-green tw-underline" href="mailto:muster@asymbl.com"
          >muster@asymbl.com</a
        >.
      </p>
      <p>
        Muster reads free/busy and event times only to compute when a group
        overlaps. It does not sell personal data.
      </p>
      <p>
        You can disconnect a calendar in Settings. You can also revoke Muster
        from your Google or Microsoft account.
      </p>
      <p>
        Muster is hosted at muster.asymbl.app. Calendar access is read-only.
      </p>
    </div>
  </LegalLayout>
</template>

<script>
import LegalLayout from "@/components/LegalLayout.vue"

export default {
  name: "PrivacyPolicy",
  components: { LegalLayout },
  metaInfo: {
    title: "Privacy policy - Asymbl Muster",
  },
}
</script>
