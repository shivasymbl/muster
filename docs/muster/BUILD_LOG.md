# Muster build log

## 2026-09-29 · Phase 0 · Upstream image build

- Symptom: none. Gate check.
- Root cause: n/a
- Fix: `docker compose build` succeeded for `timeful-frontend` and `timeful-server` on the untouched upstream tree (`cd03b3fc`) before brand edits. Frontend context was sent before any Muster file changes.
- Source: local Docker build.

## 2026-09-29 · Phase 1 · Pronunciation assets

- Symptom: `PronunciationMenu.vue` is the only importer of `assets/audio/` and `assets/doodles/pronunciation/`. Deleting those assets in Phase 1 would fail `npm run build` while the component remained.
- Root cause: the component exists only to teach the Schej pronunciation and is removed by the spec in Phase 2.
- Fix: deleted `PronunciationMenu.vue` and its unused registration in `Landing.vue` in the brand commit, together with the audio and doodle assets.
- Source: spec sections 6.4 and 7.1.

## 2026-09-29 · Phase 1 · Vuetify font variable

- Symptom: `npm run build` failed with 195 `SassError: Expected newline` errors. The injected source started with `(content, loaderContext) => {`.
- Root cause: this project's sass-loader treats `css.loaderOptions.sass.additionalData` as a string. A function was coerced into the indented `.sass` files. Vuetify component styles are indented Sass, so an SCSS `@import` is also invalid there.
- Fix: `frontend/src/sass/variables.sass` sets `$body-font-family`, and `vue.config.js` prepends `@import "@/sass/variables.sass"` as a string. `@font-face` src paths are relative to `public/fonts` so css-loader can resolve them.
- Source: local build error. Vuetify 2.7 ships indented Sass. css-loader 6 treats absolute `/fonts/...` urls as modules.

## 2026-09-29 · Phase 2 · Google contacts scopes

- Symptom: spec section 7.4 says to drop `contacts.readonly` and `directory.readonly` unless a kept feature needs them.
- Root cause: `server/services/contacts/contacts.go` and `GET` contact search in `server/routes/user.go` still search Google contacts and the directory. Sign-in only requests those scopes when `requestContactsPermission` is true.
- Fix: left the scopes in place so that feature still works. Needs a decision from Shiv before they are removed.
- Source: spec section 7.4.

## 2026-09-29 · Phase 2 · Historical Go scripts

- Symptom: `go build ./...` failed in three `server/scripts/*` mains on fields the current models no longer have (`Responses`, old calendar token fields).
- Root cause: those files are one-off upstream migrations and already did not match `models.Event` / `models.CalendarAccount` before the Muster edits.
- Fix: added `//go:build ignore` so the package build skips them. They are not part of the server.
- Source: `go build ./...` error output.

## 2026-09-29 · Phase 3 · Privacy page

- The privacy page is a plain-language draft. Asymbl legal still needs to review it before it is treated as the final policy.
- Controller: Asymbl Inc. Contact: muster@asymbl.com.

## 2026-09-29 · Phase 5 · OAuth redirect URIs

- Google and Microsoft both use `${window.location.origin}/auth` (`frontend/src/utils/sign_in_utils.js`). The server exchanges the code with the same origin (`server/services/auth/auth.go`).
- Production redirect URI: `https://muster.asymbl.app/auth`
- Authorized JavaScript origin: `https://muster.asymbl.app`
- Google scopes still include `contacts.readonly` and `directory.readonly` because contact search is still in the product. Waiting on a decision before dropping them.
- Logo for the consent screen: `frontend/src/assets/brand/muster-mark-1024.png`

## 2026-09-29 · Phase 5 · Image smoke test and DNS

- `docker build -t muster:local .` succeeded. `GET /api/health` returned `{"ok":true}` with the image pointed at a local Mongo 7 container. The containers were removed after the check.
- `dig NS asymbl.app` returns Cloudflare (`maciej.ns.cloudflare.com`, `liv.ns.cloudflare.com`). The zone is not on DigitalOcean, so no DigitalOcean DNS record was created.
- Depot, DigitalOcean, Google, and Microsoft credentials are not in the environment. Droplet creation and the Depot push are waiting on those values. The A record cannot be handed over until the droplet IP exists.

## 2026-09-30 · Phase 5 · Depot and gcloud CLIs

- Depot CLI 2.101.65 was already signed in to organization Asymbl Inc (`p6gdqmvg63`). Created project `muster` with id `96s5kdtrsn` in `us-east-1`.
- `depot init --project` does not apply the flag in this CLI build (it only prompts on a terminal). Wrote `depot.json` as `{"projectID":"96s5kdtrsn"}`, which matches the field this binary unmarshals. Not committed yet.
- gcloud was already signed in as `sdevinarayanan@asymbl.com`. The active project was left as `asymbl-langfuse-prod`.
- Created GCP project `asymbl-muster` (number `618624586718`, display name Asymbl Muster) under organization `asymbl.com` (`989529849740`). Calendar, People, and Admin SDK APIs are not enabled yet, and the OAuth client is not created.

## 2026-09-30 · Phase 5 · Google APIs enabled

- Enabled `calendar-json.googleapis.com`, `people.googleapis.com`, and `admin.googleapis.com` on `asymbl-muster`. The active gcloud project was left as `asymbl-langfuse-prod`.
- Contact and directory scopes stay, because contact search still uses them.
- A user OAuth web client cannot be created from gcloud. The IAP OAuth Admin API was shut down on 2026-03-19, and `gcloud iam oauth-clients create` only allows Cloud Platform, openid, email, and groups scopes. The client has to be created in the Google Auth Platform console.

## 2026-09-30 · Phase 5 · Google OAuth client created

- Consent screen for `asymbl-muster` is External and in Testing. App name Asymbl Muster. Support and developer contact are `sdevinarayanan@asymbl.com`. Home `https://muster.asymbl.app`. Privacy `https://muster.asymbl.app/privacy-policy`. Authorized domain `asymbl.app`.
- Web client name: Asymbl Muster Web. Client ID `618624586718-bormha34mt4240urc23if9c6vathqfmn.apps.googleusercontent.com`.
- Redirect URIs in the downloaded client: `https://muster.asymbl.app/auth` and `http://localhost:8080/auth`. JavaScript origin saved: `https://muster.asymbl.app` only.
- Scopes were clicked saved as openid, userinfo.email, userinfo.profile, calendar.calendarlist.readonly, calendar.events.readonly, contacts.readonly, and directory.readonly. The Data Access page was not reloaded to confirm the save.
- Logo was not uploaded. Test-user list is empty.
- Client ID and secret are in gitignored `server/.env`. The public client ID is in gitignored `frontend/.env.local`. Local `SESSION_SECRET` and `ENCRYPTION_KEY` were generated for this machine only. Nothing from the client JSON was committed.

## 2026-09-30 · Phase 5 · Production droplet

- Droplet `muster-prod-01` (`604873110`) in `nyc3`, size `s-2vcpu-4gb`, Ubuntu 24.04, monitoring and backups on. Public IP `157.245.124.63`.
- Volume `muster-data` (`722cbd60-bc93-11f1-b0d9-9a56f5d022db`, 20 GiB) is mounted at `/mnt/muster_data`.
- Firewall `muster-prod` allows TCP 22, 80, and 443 from the public internet. SSH is key-only.
- Cloudflare A record `muster.asymbl.app` points at `157.245.124.63`, TTL 300, DNS-only.
- Caddy obtained a Let's Encrypt certificate. The first start failed because `/var/log/caddy` was not writable by the `caddy` user. That directory is now owned by `caddy`, and cloud-init creates it that way.
- Image `registry.depot.dev/96s5kdtrsn:32560c3c` is running. `GET https://muster.asymbl.app/api/health` returns `{"ok":true}`.
- `/privacy-policy` and `/terms` are static HTML so a browser that does not run JavaScript, including Google's branding check, can read them. The in-app pages are the same copy.
- Production `SESSION_SECRET` and `ENCRYPTION_KEY` were generated on the droplet. They are not the local values. Microsoft OAuth is still unset. The Depot pull token was removed from the droplet after the image pull.

## 2026-09-30 · Phase 5 · Consent logo and domain check

- Google branding still rejects the letter-only mark and says `muster.asymbl.app` is not registered to the project owner.
- Drew an opaque 120×120 consent logo that reads “muster.” with “asymbl” under it: `frontend/src/assets/brand/muster-consent-120.png`. The app icon is unchanged.
- `asymbl.app` has no `google-site-verification` TXT record. The only owner of `asymbl-muster` is `sdevinarayanan@asymbl.com`. The gcloud token does not include the Search Console scope, so the DNS token has to come from that account in Search Console.
- Search Console later reported Ownership verified for the domain property, method Domain name provider. The apex TXT `google-site-verification` record is present and must stay.
- The consent logo `muster-consent-120.png` was uploaded. Google Auth Platform reports branding verified and shown to users.
- Direct loads of `/privacy-policy` and `/terms` are branded static pages (wordmark, Inter, ink and blue) so they match the home page without requiring JavaScript.




