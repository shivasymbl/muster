# Verification

Recorded 2026-09-29. Production at https://muster.asymbl.app is not up yet, so the section 11 checklist has not been run against the live site.

## Done locally

- Frontend production build on Node 18 succeeded after the rebrand and after the monetization removal.
- `go build ./...` in `server/` succeeded. `GET /api/health` is registered.
- Landing page screenshots at 1440, 768, and 390 are in `docs/muster/screenshots/`. At 1440 the page width matched the viewport (no horizontal overflow). "Start a Muster" opens the new-event dialog.
- Production image `muster:local` built from the root Dockerfile. Against a temporary Mongo 7 container, `/api/health` returned `{"ok":true}`.
- Pull request: https://github.com/shivasymbl/muster/pull/1

## Blocked before Gate 5

- `DEPOT_TOKEN`, `DO_TOKEN`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `MICROSOFT_CLIENT_ID`, and `MICROSOFT_CLIENT_SECRET` are not set.
- `asymbl.app` nameservers are Cloudflare, not DigitalOcean. After the droplet exists, add `A muster <droplet-ip> TTL 300` and leave it DNS-only (grey cloud).
- OAuth apps were not created from here. Redirect URI for both providers: `https://muster.asymbl.app/auth`. JavaScript origin: `https://muster.asymbl.app`.

## Not run

Sign-in, calendar overlap, time zones, unlimited events, settings disconnect, Lighthouse, and view-source on the production host. Do not merge the pull request or tag `v1.0.0-muster` until those pass.
