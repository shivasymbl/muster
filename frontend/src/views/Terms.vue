<template>
  <LegalLayout>
    <h1 class="tw-mt-2 tw-text-4xl tw-font-semibold">Terms of service</h1>
    <p
      class="tw-mt-4 tw-inline-block tw-rounded-full tw-bg-ligher-green tw-px-3 tw-py-2 tw-text-sm tw-text-slate"
    >
      This page needs review by Asymbl legal before it is treated as final.
    </p>
    <div class="tw-mt-8 tw-space-y-4 tw-leading-relaxed">
      <p>
        Asymbl Muster is a free group scheduling service from Asymbl Inc. By
        using https://muster.asymbl.app you agree to these terms. Questions go
        to
        <a class="tw-text-green tw-underline" href="mailto:muster@asymbl.com"
          >muster@asymbl.com</a
        >.
      </p>
      <p>
        You may create events, invite people, and connect a calendar so Muster
        can show when the group overlaps. Calendar access is read-only. You
        can disconnect a calendar in Settings or revoke Muster from your
        Google or Microsoft account.
      </p>
      <p>
        Do not use Muster to break the law, disrupt the service, or collect
        other people's data beyond scheduling the event you created. Asymbl
        may suspend access that harms the service or other people.
      </p>
      <p>
        Muster is provided as available, without a warranty. Asymbl is not
        liable for missed meetings or loss of data. The source is open under
        AGPL-3.0. These terms apply to the hosted service at
        muster.asymbl.app.
      </p>
    </div>
  </LegalLayout>
</template>

<script>
import LegalLayout from "@/components/LegalLayout.vue"

export default {
  name: "Terms",
  components: { LegalLayout },
  metaInfo: {
    title: "Terms of service - Asymbl Muster",
  },
}
</script>
