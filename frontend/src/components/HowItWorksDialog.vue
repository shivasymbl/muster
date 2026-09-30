<template>
  <v-dialog v-model="dialog" max-width="32rem">
    <v-card class="tw-p-2">
      <v-card-title class="tw-text-xl tw-font-medium">How Muster works</v-card-title>
      <v-card-text class="tw-text-very-dark-gray">
        <ol class="tw-list-decimal tw-space-y-2 tw-pl-5">
          <li>Create a Muster and pick the dates and hours you want to offer.</li>
          <li>Share one link in email, WhatsApp, Slack, or Teams.</li>
          <li>See the overlap, pick the slot, and send the invite.</li>
        </ol>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn text @click="dialog = false">Close</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script>
export default {
  name: "HowItWorksDialog",
  props: {
    value: Boolean,
  },
  computed: {
    dialog: {
      get() {
        return this.value
      },
      set(val) {
        this.$emit("input", val)
      },
    },
  },
}
</script>
