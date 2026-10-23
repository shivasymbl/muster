<template>
  <div class="tw-overflow-x-hidden tw-bg-white tw-text-ink">
    <header
      class="tw-sticky tw-top-0 tw-z-20 tw-border-b tw-border-light-gray-stroke tw-bg-white/95 tw-backdrop-blur"
    >
      <div
        class="tw-mx-auto tw-flex tw-max-w-6xl tw-items-center tw-gap-3 tw-px-4 tw-py-3 sm:tw-px-6"
      >
        <router-link :to="authUser ? { name: 'home' } : { name: 'landing' }">
          <Logo color="navy" :width="118" />
        </router-link>
        <v-spacer />
        <a
          href="#how"
          class="tw-hidden tw-text-sm tw-font-medium tw-text-ink sm:tw-inline"
          >How it works</a
        >
        <router-link
          :to="{ name: 'privacy-policy' }"
          class="tw-hidden tw-text-sm tw-font-medium tw-text-ink sm:tw-inline"
          >Privacy</router-link
        >
        <router-link
          :to="{ name: 'terms' }"
          class="tw-hidden tw-text-sm tw-font-medium tw-text-ink sm:tw-inline"
          >Terms</router-link
        >
        <AuthUserMenu v-if="authUser" class="tw-ml-1" />
        <v-btn
          v-else
          text
          class="tw-text-ink"
          :to="{ name: 'sign-in' }"
          >Sign in</v-btn
        >
        <v-btn
          v-if="authUser"
          outlined
          class="tw-rounded-lg tw-px-4 tw-text-sm"
          color="primary"
          :to="{ name: 'home' }"
          >Home</v-btn
        >
        <v-btn
          class="tw-rounded-lg tw-bg-green tw-px-4 tw-text-sm"
          dark
          @click="startMuster"
          >Schedule a meeting</v-btn
        >
      </div>
    </header>

    <main>
      <section class="tw-px-4 tw-py-8 sm:tw-px-6 sm:tw-py-12">
        <div
          class="tw-mx-auto tw-grid tw-w-full tw-max-w-6xl tw-items-center tw-gap-10 tw-rounded-3xl tw-px-6 tw-py-12 sm:tw-px-12 lg:tw-grid-cols-2 lg:tw-py-16"
          style="background: linear-gradient(180deg, #191d47, #2c1169)"
        >
          <div class="tw-min-w-0 tw-text-white">
            <div
              class="tw-mb-4 tw-inline-flex tw-rounded-full tw-border tw-border-white/30 tw-px-3 tw-py-1 tw-text-sm"
            >
              Scheduling for any meeting
            </div>
            <h1 class="tw-text-4xl tw-font-semibold tw-leading-tight sm:tw-text-5xl">
              Book the meeting in
              <span style="color: #b2deff">one link</span>.
            </h1>
            <p class="tw-mt-4 tw-text-base tw-leading-relaxed tw-text-white/90 sm:tw-text-lg">
              Send one link. They tap the times they can make, no accounts and
              no installs. A client interview, a hiring-manager meeting, a
              sales call, or any other meeting. You see the overlap and book it.
            </p>
            <div class="tw-mt-8">
              <v-btn
                class="tw-rounded-lg tw-bg-green tw-px-6"
                dark
                large
                @click="startMuster"
                >Schedule a meeting</v-btn
              >
              <div class="tw-mt-3 tw-flex tw-flex-wrap tw-gap-x-4 tw-gap-y-1 tw-text-sm">
                <button
                  type="button"
                  class="tw-text-white/80 tw-underline"
                  @click="_signIn(calendarTypes.GOOGLE)"
                >
                  Sign in with Google
                </button>
                <button
                  type="button"
                  class="tw-text-white/80 tw-underline"
                  @click="_signIn(calendarTypes.OUTLOOK)"
                >
                  Sign in with Microsoft
                </button>
              </div>
            </div>
          </div>
          <div class="tw-min-w-0 tw-rounded-2xl tw-bg-white tw-p-6 tw-text-ink">
            <p class="tw-text-xs tw-font-semibold tw-uppercase tw-tracking-wide tw-text-green">
              This week's panel
            </p>
            <ul class="tw-mt-4 tw-divide-y tw-divide-light-gray-stroke">
              <li
                v-for="item in panel"
                :key="item.title"
                class="tw-flex tw-items-center tw-justify-between tw-gap-4 tw-py-3"
              >
                <div class="tw-min-w-0">
                  <p class="tw-font-semibold">{{ item.title }}</p>
                  <p class="tw-text-sm tw-text-slate">{{ item.who }}</p>
                </div>
                <span class="tw-shrink-0 tw-text-sm tw-font-medium tw-text-green">{{ item.when }}</span>
              </li>
            </ul>
          </div>
        </div>
      </section>

      <section id="how" class="tw-mx-auto tw-max-w-6xl tw-px-4 tw-py-16 sm:tw-px-6">
        <h2 class="tw-text-3xl tw-font-semibold">
          How it <span class="tw-text-green">works</span>
        </h2>
        <div class="tw-mt-8 tw-grid tw-gap-4 md:tw-grid-cols-3">
          <article
            v-for="step in steps"
            :key="step.title"
            class="tw-rounded-2xl tw-border tw-border-light-gray-stroke tw-bg-white tw-p-6"
          >
            <div
              class="tw-mb-4 tw-flex tw-h-10 tw-w-10 tw-items-center tw-justify-center tw-rounded-full tw-text-sm tw-font-semibold tw-text-white"
              :style="{ background: step.color }"
            >
              {{ step.n }}
            </div>
            <h3 class="tw-text-lg tw-font-semibold">{{ step.title }}</h3>
            <p class="tw-mt-2 tw-text-slate">{{ step.body }}</p>
          </article>
        </div>
      </section>

      <section class="tw-bg-off-white tw-px-4 tw-py-16 sm:tw-px-6">
        <div class="tw-mx-auto tw-max-w-6xl">
          <h2 class="tw-text-3xl tw-font-semibold">
            See the <span class="tw-text-green">overlap</span>
          </h2>
          <p class="tw-mt-2 tw-text-slate">
            Darker blue means more people are free. Each meeting sits on a slot they can all make.
          </p>
          <div class="tw-mt-6 tw-flex tw-justify-center tw-overflow-x-auto">
            <LandingPageCalendar />
          </div>
        </div>
      </section>

      <section class="tw-mx-auto tw-max-w-6xl tw-px-4 tw-py-16 sm:tw-px-6">
        <div
          class="tw-grid tw-gap-6 tw-rounded-3xl tw-bg-ligher-green tw-p-8 md:tw-grid-cols-2 md:tw-p-12"
        >
          <div>
            <h2 class="tw-text-3xl tw-font-semibold">
              Built for <span class="tw-text-green">agencies</span>
            </h2>
            <p class="tw-mt-4 tw-text-lg tw-leading-relaxed">
              The same link works for any meeting. A candidate, a hiring
              manager, or a sales prospect taps times on their phone, or
              connects a calendar. Your logo is what they see on the link.
            </p>
          </div>
          <ul class="tw-self-center tw-space-y-2 tw-text-lg">
            <li>Client interviews and hiring-manager meetings</li>
            <li>Sales calls</li>
            <li>Client intake and candidate prep</li>
            <li>Onboarding calls and contractor check-ins</li>
          </ul>
        </div>
      </section>

      <section class="tw-mx-auto tw-max-w-6xl tw-px-4 tw-pb-16 sm:tw-px-6">
        <div class="tw-grid tw-items-center tw-gap-8 md:tw-grid-cols-[1fr_auto]">
          <div>
            <h2 class="tw-text-3xl tw-font-semibold">
              Privacy <span class="tw-text-green">first</span>
            </h2>
            <div class="tw-mt-6 tw-flex tw-flex-wrap tw-gap-3">
              <span
                v-for="pill in privacyPills"
                :key="pill"
                class="tw-rounded-full tw-border tw-border-light-gray-stroke tw-px-4 tw-py-2 tw-text-sm tw-font-medium"
                >{{ pill }}</span
              >
            </div>
          </div>
          <img
            src="@/assets/brand/muster-consent-120.png"
            alt="Asymbl Muster"
            class="tw-mx-auto tw-h-28 tw-w-28 tw-rounded-3xl"
          />
        </div>
      </section>

      <section class="tw-mx-auto tw-max-w-3xl tw-px-4 tw-pb-16 sm:tw-px-6">
        <h2 class="tw-mb-6 tw-text-center tw-text-3xl tw-font-semibold">
          Questions
        </h2>
        <div class="tw-grid tw-gap-3">
          <FAQ
            v-for="faq in faqs"
            :key="faq.question"
            @signIn="signIn"
            v-bind="faq"
          />
        </div>
      </section>

      <section class="tw-px-4 tw-pb-16 sm:tw-px-6">
        <div
          class="tw-mx-auto tw-max-w-6xl tw-rounded-3xl tw-px-8 tw-py-12 tw-text-center tw-text-white"
          style="background: linear-gradient(90deg, #008ff8, #8855ff)"
        >
          <h2 class="tw-text-3xl tw-font-semibold sm:tw-text-4xl">
            Book the meeting without the email ping-pong
          </h2>
          <v-btn
            class="tw-mt-8 tw-rounded-lg tw-bg-white tw-px-8 tw-text-ink"
            large
            @click="startMuster"
            >Schedule a meeting</v-btn
          >
        </div>
      </section>
    </main>

    <Footer />

    <SignInDialog
      v-model="signInDialog"
      @signIn="_signIn"
      @emailSignIn="_emailSignIn"
    />
    <NewDialog v-model="newDialog" no-tabs @signIn="signIn" />
  </div>
</template>

<script>
import LandingPageCalendar from "@/components/landing/LandingPageCalendar.vue"
import { signInGoogle, signInOutlook } from "@/utils"
import FAQ from "@/components/FAQ.vue"
import NewDialog from "@/components/NewDialog.vue"
import Logo from "@/components/Logo.vue"
import SignInDialog from "@/components/SignInDialog.vue"
import { calendarTypes } from "@/constants"
import Footer from "@/components/Footer.vue"
import { mapState, mapMutations } from "vuex"
import AuthUserMenu from "@/components/AuthUserMenu.vue"

export default {
  name: "Landing",

  metaInfo: {
    title: "Asymbl Muster: book the meeting",
  },

  components: {
    LandingPageCalendar,
    FAQ,
    NewDialog,
    Logo,
    SignInDialog,
    Footer,
    AuthUserMenu,
  },

  data: () => ({
    signInDialog: false,
    newDialog: false,
    calendarTypes,
    panel: [
      { title: "Recruiter screen", who: "Priya Shah, candidate", when: "30 min" },
      { title: "Hiring manager", who: "Alex Chen, engineering", when: "45 min" },
      { title: "Client interview", who: "Northwind team", when: "60 min" },
      { title: "Offer call", who: "Priya and the recruiter", when: "20 min" },
    ],
    steps: [
      {
        n: "1",
        color: "#038FF8",
        title: "Set the window",
        body: "Pick the weekdays and hours. Start from an interview, a hiring-manager meeting, a sales call, or any other meeting.",
      },
      {
        n: "2",
        color: "#ED489E",
        title: "Send one link",
        body: "They tap their times. No sign-up. Or they connect a calendar. Nobody installs anything.",
      },
      {
        n: "3",
        color: "#8856FF",
        title: "Book the overlap",
        body: "You see the slots everyone can make, across time zones, and book the meeting.",
      },
    ],
    privacyPills: [
      "Free/busy only",
      "Read-only access",
      "Revoke anytime",
      "No data selling",
    ],
    faqs: [
      {
        question: "Is this only for interviews?",
        answer:
          "No. The same link works for any meeting: a client interview, a meeting with a hiring manager, a sales call, an intake, or anything else you need on the calendar.",
      },
      {
        question: "Does the other person need an account?",
        answer:
          "No. A candidate, a client, or a prospect opens the link and taps the times they can make. No sign-up and no calendar connection.",
      },
      {
        question: "Does anyone need to install anything?",
        answer:
          "No. They open the link in a browser. They can tap times, or connect Google or Outlook if they want the grid filled from their calendar.",
      },
      {
        question: "Does it handle time zones?",
        answer:
          "Yes. Each person sees the grid in their own time zone. You can note someone's time zone on the meeting.",
      },
      {
        question: "Does it work with Bullhorn or Salesforce?",
        answer:
          'Not yet. Muster does not write meetings back to an ATS or a CRM. Want booked interviews written back to the candidate record in Salesforce? <a href="mailto:muster@asymbl.com">Talk to Asymbl</a>.',
      },
      {
        question: "Is Muster free?",
        answer:
          "Yes. There is no event limit and no paid tier.",
      },
      {
        question: "Which calendars work?",
        answer:
          "Google Calendar, Outlook, and Apple Calendar or iCloud. Anyone can skip the calendar and tap times instead.",
      },
    ],
  }),

  computed: {
    ...mapState(["authUser"]),
  },

  methods: {
    ...mapMutations(["setAuthUser"]),
    startMuster() {
      this.newDialog = true
    },
    _signIn(calendarType) {
      if (calendarType === calendarTypes.GOOGLE) {
        signInGoogle({ state: null, selectAccount: true })
      } else if (calendarType === calendarTypes.OUTLOOK) {
        signInOutlook({ state: null, selectAccount: true })
      }
    },
    _emailSignIn(user) {
      this.setAuthUser(user)
      this.$posthog?.identify(user._id, {
        email: user.email,
        firstName: user.firstName,
        lastName: user.lastName,
      })
      this.$router.replace({ name: "home" })
    },
    signIn() {
      this.$router.push({ name: "sign-in" })
    },
  },
}
</script>
