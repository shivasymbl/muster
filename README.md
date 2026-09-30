<p align="center">
  <img src="./frontend/src/assets/brand/muster-wordmark-navy.png" width="240" alt="Asymbl Muster" />
</p>

<p align="center">
  Scheduling for recruiting teams. One link for the candidate, the hiring manager, and the client.
</p>

<p align="center">
  <a href="https://muster.asymbl.app">muster.asymbl.app</a>
</p>

<p align="center">
  <img src="./frontend/src/assets/brand/muster-consent-120.png" width="96" alt="Muster mark" />
</p>

## What it is for

Muster is the overlap grid for a hiring process. A recruiter sets the dates and hours. The candidate, the hiring manager, and the client each connect their own calendar. Muster shows the slots the whole panel can make.

Nobody shares a calendar with another company. Muster reads free/busy only.

## A panel, not a party

The sample week on the site is a recruiting week:

- Recruiter screen
- Hiring manager interview
- Client interview
- Offer call

## How a search uses it

1. Set the interview window.
2. Send one link.
3. Book the overlap and send the invite.

Google Calendar, Outlook, and Apple Calendar. Availability can also be entered by hand.

## Stack

Vue 2, Vuetify 2, Tailwind 3, Go (Gin), MongoDB 7. Production is one image on a DigitalOcean droplet, built with Depot. See [DEPLOYMENT.md](./DEPLOYMENT.md).

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

Set `SESSION_SECRET` (at least 32 characters), `ENCRYPTION_KEY`, and the Google OAuth client values before signing in. `BASE_URL` defaults to `https://muster.asymbl.app`. For local links, set `BASE_URL=http://localhost:8080` and `CORS_ORIGINS=http://localhost:8080`.

## Email

Join notifications and reminders go out only when an email provider is configured. Listmonk is the upstream mailer and is off unless `LISTMONK_URL` is set. Asymbl's domain already uses Resend, which is the provider to wire up for Muster mail.

## License

Muster is licensed under AGPL-3.0. The hosted service links to this source from the footer.

## Credits

Muster is a fork of [Timeful](https://github.com/schej-it/timeful.app) by the Schej team, used under AGPL-3.0.
