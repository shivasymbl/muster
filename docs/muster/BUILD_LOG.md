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

