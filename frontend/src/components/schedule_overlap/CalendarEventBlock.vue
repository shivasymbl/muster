<template>
  <transition :name="transitionName" appear>
    <div
      class="tw-absolute tw-w-full tw-select-none tw-overflow-hidden tw-p-px"
      :style="blockStyle"
      style="pointer-events: none"
    >
      <div
        class="tw-h-full tw-w-full tw-overflow-hidden tw-rounded tw-border tw-border-solid tw-bg-white tw-px-1 tw-text-xs tw-leading-tight"
        :class="containerClass"
      >
        <div :class="textColor" class="ph-no-capture tw-truncate tw-font-medium">
          {{ noEventNames ? "BUSY" : calendarEvent.summary }}
        </div>
      </div>
    </div>
  </transition>
</template>

<script>
export default {
  name: "CalendarEventBlock",
  props: {
    blockStyle: {
      type: Object,
      default: () => ({}),
    },
    calendarEvent: {
      type: Object,
      required: true,
    },
    isGroup: {
      type: Boolean,
      required: true,
    },
    isEditingAvailability: {
      type: Boolean,
      required: true,
    },
    noEventNames: {
      type: Boolean,
      required: true,
    },
    transitionName: {
      type: String,
      required: true,
    },
  },
  computed: {
    containerClass() {
      if (this.calendarEvent.free) {
        return "tw-border-dashed tw-border-ink"
      }
      return "tw-border-ink"
    },
    textColor() {
      return "tw-text-ink"
    },
  },
}
</script>
