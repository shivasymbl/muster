<template>
  <v-card class="tw-m-0 tw-w-full tw-max-w-xl tw-rounded-lg">
    <div>
      <ScheduleOverlap
        ref="scheduleOverlap"
        :event="event"
        :sampleCalendarEventsByDay="calendarEventsByDay"
        calendarOnly
        :interactable="false"
        :showSnackbar="false"
        :alwaysShowCalendarEvents="true"
        :showHintText="false"
      />
    </div>
  </v-card>
</template>

<script>
import ScheduleOverlap from "@/components//schedule_overlap/ScheduleOverlap"
import { getDateWithTimeNum, processTimeBlocks } from "@/utils"

function upcomingWeekdays(count) {
  const days = []
  const cursor = new Date()
  cursor.setHours(12, 0, 0, 0)
  while (days.length < count) {
    const dow = cursor.getDay()
    if (dow !== 0 && dow !== 6) days.push(new Date(cursor))
    cursor.setDate(cursor.getDate() + 1)
  }
  return days
}

function slots(day, start, end) {
  const times = []
  for (let t = start; t < end; t += 0.5) {
    times.push(getDateWithTimeNum(day, t))
  }
  return times
}

export default {
  name: "LandingPageCalendar",

  components: {
    ScheduleOverlap,
  },

  data() {
    const days = upcomingWeekdays(3).map((day) => getDateWithTimeNum(day, 9))
    return {
      dates: days,
      responses: {},
      calendarEventsByDay: [],
    }
  },

  computed: {
    event() {
      // startTime is a UTC hour. Keep the half hour in zones like India so
      // the first row is 9:00 local instead of a blank half hour.
      const offsetMin = new Date().getTimezoneOffset()
      let startTime = 9 + offsetMin / 60
      startTime = ((startTime % 24) + 24) % 24
      return {
        dates: this.dates,
        duration: 8,
        startTime,
        endTime: startTime + 8,
        responses: this.responses,
      }
    },
  },

  methods: {
    getCalendarEventsByDay() {
      const [day1, day2, day3] = this.dates
      const events = [
        {
          startDate: getDateWithTimeNum(day1, 9),
          endDate: getDateWithTimeNum(day1, 9.5),
          summary: "Recruiter screen",
        },
        {
          startDate: getDateWithTimeNum(day1, 11),
          endDate: getDateWithTimeNum(day1, 11 + 1 / 3),
          summary: "Offer call",
        },
        {
          startDate: getDateWithTimeNum(day2, 13),
          endDate: getDateWithTimeNum(day2, 13.75),
          summary: "Hiring manager interview",
        },
        {
          startDate: getDateWithTimeNum(day3, 10),
          endDate: getDateWithTimeNum(day3, 11),
          summary: "Client interview",
        },
      ]

      this.calendarEventsByDay = processTimeBlocks(this.dates, 8, events)
    },
    getResponses() {
      const [day1, day2, day3] = this.dates
      const availability = [
        ...slots(day1, 9, 12),
        ...slots(day2, 13, 16),
        ...slots(day3, 10, 12),
      ]
      const expanded = []
      for (const date of availability) {
        expanded.push(date)
        const next = new Date(date)
        next.setMinutes(date.getMinutes() + 15)
        expanded.push(next)
      }
      this.responses = {
        recruiter: {
          user: { _id: "recruiter", name: "Recruiter" },
          availability: expanded,
        },
        candidate: {
          user: { _id: "candidate", name: "Candidate" },
          availability: expanded,
        },
        client: {
          user: { _id: "client", name: "Client" },
          availability: expanded,
        },
      }
    },
  },

  created() {
    this.getCalendarEventsByDay()
    this.getResponses()
  },
}
</script>
