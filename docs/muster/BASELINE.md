# Upstream baseline

Recorded 2026-09-29 from `schej-it/timeful.app` at `cd03b3fc` (`upstream-base-20260929`), before any Muster changes.

## String counts

Commands were run from the repo root.

- `rg -io "timeful|schej" frontend/src server | wc -l` → **338**
- `rg -il "timeful|schej" frontend/src server | wc -l` → **97 files**

## Monetization, ads, and community files

`rg -il "premium|upgrade|stripe|donate|publift|carbon|adsense|discord|reddit|paypal" frontend server | sort`

- frontend/package-lock.json
- frontend/public/index.html
- frontend/src/App.vue
- frontend/src/components/CookieSettings.vue
- frontend/src/components/DiscordBanner.vue
- frontend/src/components/EventType.vue
- frontend/src/components/Footer.vue
- frontend/src/components/TeamsNotReadyDialog.vue
- frontend/src/components/UpvoteRedditSnackbar.vue
- frontend/src/components/event/CarbonAd.vue
- frontend/src/components/event/PubliftAd.vue
- frontend/src/components/home/Dashboard.vue
- frontend/src/components/pricing/AlreadyDonatedDialog.vue
- frontend/src/components/pricing/UpgradeDialog.vue
- frontend/src/components/schedule_overlap/ScheduleOverlap.vue
- frontend/src/components/schedule_overlap/ToolRow.vue
- frontend/src/constants.js
- frontend/src/router/index.js
- frontend/src/store/index.js
- frontend/src/utils/general_utils.js
- frontend/src/views/Auth.vue
- frontend/src/views/Event.vue
- frontend/src/views/Landing.vue
- frontend/src/views/Settings.vue
- frontend/src/views/SignIn.vue
- frontend/src/views/StripeRedirect.vue
- frontend/src/views/Test.vue
- frontend/tailwind.config.js
- server/db/users.go
- server/discord_bot/commands/active_users.go
- server/discord_bot/commands/help.go
- server/discord_bot/commands/index.go
- server/discord_bot/commands/num_users.go
- server/discord_bot/init.go
- server/docs/docs.go
- server/docs/swagger.json
- server/docs/swagger.yaml
- server/go.mod
- server/go.sum
- server/main.go
- server/models/user.go
- server/routes/analytics.go
- server/routes/events.go
- server/routes/stripe.go
- server/routes/users.go
