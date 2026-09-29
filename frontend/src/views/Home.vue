<template>
  <span>
    <div
      class="tw-mx-auto tw-mb-24 tw-mt-4 tw-max-w-6xl tw-space-y-4 sm:tw-mb-12 sm:tw-mt-7"
    >
      <div
        v-if="loading && !eventsNotEmpty"
        class="tw-flex tw-h-[calc(100vh-10rem)] tw-w-full tw-items-center tw-justify-center"
      >
        <v-progress-circular
          indeterminate
          color="primary"
          :size="20"
          :width="2"
        ></v-progress-circular>
      </div>

      <v-fade-transition>
        <Dashboard v-if="!loading || eventsNotEmpty" />
      </v-fade-transition>

      <div
        class="tw-rounded-md tw-px-6 tw-py-4 sm:tw-mx-4 sm:tw-bg-[#f3f3f366]"
        v-if="!loading || eventsNotEmpty"
      >
        <div
          class="tw-mb-3 tw-text-xl tw-font-medium tw-text-dark-green sm:tw-text-2xl"
        >
          Tools
        </div>
        <div class="tw-flex tw-flex-row tw-items-center tw-gap-2">
          <div
            @click="convertW2M"
            class="tw-cursor-pointer tw-text-sm tw-font-normal tw-text-dark-gray tw-underline"
          >
            Convert When2meet
          </div>
        </div>
      </div>

      <div v-if="!loading || eventsNotEmpty" class="tw-flex tw-justify-center">
        <img
          src="@/assets/brand/illustrations/robot-thumbs-up.png"
          alt=""
          class="tw-h-48 tw-w-auto"
        />
      </div>

      <div class="tw-flex tw-flex-col tw-items-center tw-justify-between">
        <router-link
          class="tw-text-xs tw-font-medium tw-text-gray"
          :to="{ name: 'privacy-policy' }"
        >
          Privacy Policy
        </router-link>
      </div>

      <!-- FAB -->
      <BottomFab
        v-if="isPhone"
        id="create-event-btn"
        @click="() => _createNew()"
      >
        <v-icon>mdi-plus</v-icon>
      </BottomFab>

      <!-- When2meet Import Dialog -->
      <When2meetImportDialog v-model="showW2MDialog" />
    </div>
  </span>
</template>

<script>
import EventType from "@/components/EventType.vue"
import BottomFab from "@/components/BottomFab.vue"
import CreateSpeedDial from "@/components/CreateSpeedDial.vue"
import When2meetImportDialog from "@/components/When2meetImportDialog.vue"
import Dashboard from "@/components/home/Dashboard.vue"
import { mapState, mapActions, mapMutations } from "vuex"
import { eventTypes } from "@/constants"
import { isPhone, get } from "@/utils"

export default {
  name: "Home",

  metaInfo: {
    title: "Home - Timeful",
  },

  components: {
    EventType,
    BottomFab,
    CreateSpeedDial,
    When2meetImportDialog,
    Dashboard,
  },

  props: {
    contactsPayload: {
      type: Object,
      default: () => ({}),
    },
    openNewGroup: { type: Boolean, default: false },
  },

  data: () => ({
    loading: true,
    showW2MDialog: false,
  }),

  mounted() {
    // If coming from enabling contacts, show the dialog. Checks if contactsPayload is not an Observer.
    this.setNewDialogOptions({
      show: Object.keys(this.contactsPayload).length > 0 || this.openNewGroup,
      contactsPayload: this.contactsPayload,
      openNewGroup: this.openNewGroup,
      eventOnly: false,
    })
  },

  computed: {
    ...mapState(["events", "authUser", "groupsEnabled"]),
    eventsNotEmpty() {
      return this.events.length > 0
    },
    isPhone() {
      return isPhone(this.$vuetify)
    },
  },

  methods: {
    ...mapMutations(["setAuthUser", "setNewDialogOptions"]),
    ...mapActions(["getEvents", "createNew"]),
    userRespondedToEvent(event) {
      return event.hasResponded ?? false
    },
    _createNew() {
      this.createNew({ eventOnly: false })
    },
    createFolder() {},
    convertW2M() {
      this.showW2MDialog = true
      this.$posthog?.capture("convert_when2meet_clicked")
    },
  },

  created() {
    this.getEvents().then(() => {
      this.loading = false
    })
    get("/user/profile")
      .then((authUser) => {
        this.setAuthUser(authUser)
      })
      .catch(() => {
        this.setAuthUser(null)
      })
  },
}
</script>
