<img src="./frontend/src/assets/brand/muster-wordmark-navy.png" width="220" alt="Asymbl Muster" />

Asymbl Muster helps a group find a time that works for everyone. Send one link, everyone connects their calendar, and Muster shows the overlap. Hosted at https://muster.asymbl.app.

## Features

- See when everybody's availability overlaps
- Specify the dates and hours a group can meet
- Google Calendar, Outlook, and Apple Calendar
- Available and if-needed times
- See when a subset of people is free
- Schedule across time zones
- Email notifications and reminders, when email is configured
- Duplicate a poll
- Availability groups
- Export availability as CSV
- Show responses only to the person who created the event
- Import a When2meet poll

## Tech stack

Vue 2, Vuetify 2, Tailwind 3, Go (Gin), MongoDB 7.

## Local development

Frontend, from `frontend/` with Node 18:

```
npm ci
npm run serve
```

Server, from `server/`:

```
cp .env.template .env
go run .
```

Set `SESSION_SECRET` (at least 32 characters), `ENCRYPTION_KEY`, and the Google OAuth client values before signing in. `BASE_URL` defaults to `https://muster.asymbl.app`. For local links, set `BASE_URL=http://localhost:8080` and `CORS_ORIGINS=http://localhost:8080`. Mongo uses `MONGO_DB_NAME`, which defaults to `muster`.

`compose.yaml` starts Mongo, the frontend build, and the API for local Docker.

## Deployment

Production runs as one image on a DigitalOcean droplet, built with Depot. See [DEPLOYMENT.md](./DEPLOYMENT.md).

## License

Muster is licensed under AGPL-3.0. Anyone using the hosted service can get the source from the source-code link in the footer.

## Credits

Muster is a fork of Timeful (https://github.com/schej-it/timeful.app) by the Schej team, used under AGPL-3.0.
