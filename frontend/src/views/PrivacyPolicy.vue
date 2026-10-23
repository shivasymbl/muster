<template>
  <LegalLayout title="Privacy policy">
    <div class="tw-space-y-4">
      <p>
        Asymbl Inc is the controller for Asymbl Muster. Questions go to
        <a class="tw-text-green tw-underline" href="mailto:muster@asymbl.com"
          >muster@asymbl.com</a
        >.
      </p>
      <p>
        Muster reads free/busy and event times only to find when a hiring
        panel overlaps. An interview link can also include a candidate name,
        a client name, and a job. It does not sell personal data.
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
