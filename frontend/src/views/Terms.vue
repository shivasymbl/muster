<template>
  <LegalLayout title="Terms of service">
    <div class="tw-space-y-4">
      <p>
        Asymbl Muster is a free scheduling service from Asymbl Inc for
        recruiting teams. By using https://muster.asymbl.app you agree to these
        terms. Questions go to
        <a class="tw-text-green tw-underline" href="mailto:muster@asymbl.com"
          >muster@asymbl.com</a
        >.
      </p>
      <p>
        You may create interview windows, invite a candidate or a client, and
        connect a calendar so Muster can show when the panel overlaps. Calendar
        access is read-only. You can disconnect a calendar in Settings or
        revoke Muster from your Google or Microsoft account.
      </p>
      <p>
        Do not use Muster to break the law, disrupt the service, or collect
        other people's data beyond scheduling the interview you created. Asymbl
        may suspend access that harms the service or other people.
      </p>
      <p>
        Muster is provided as available, without a warranty. Asymbl is not
        liable for a missed interview or loss of data. The source is open under
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
