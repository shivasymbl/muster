# Deploy Asymbl Muster

Production is one container image. Depot builds it. A DigitalOcean droplet in `nyc3` runs it behind Caddy at https://muster.asymbl.app.

Local development still uses `compose.yaml`. Production uses `deploy/compose.prod.yaml`.

## Image

The root `Dockerfile` builds the Vue app with Node 18, compiles the Go API with Go 1.25, and runs both from Alpine. The API listens on port 3002 and serves `./frontend/dist`.

OAuth client IDs are Docker build args (`VUE_APP_GOOGLE_CLIENT_ID`, `VUE_APP_MICROSOFT_CLIENT_ID`). Secrets stay in `/opt/muster/.env` on the droplet and are not baked into the image.

`GET /api/health` returns 200 when the process is up.

## Depot

Create a Depot project named `muster`, commit the generated `depot.json`, and push a `linux/amd64` image to the Depot registry. Confirm the current flags with `depot build --help` before a release. Tag the image with the git short SHA and with `latest`.

The droplet pulls with a read-only Depot token. `deploy/deploy.sh` builds, pushes, pulls that SHA on the droplet, and rolls back to `/opt/muster/.last_good_sha` if health checks fail.

## Droplet

- Volume `muster-data` mounted at `/mnt/muster_data` (20 GiB)
- Droplet `muster-prod-01`, Ubuntu 24.04, 2 vCPU, 4 GB, monitoring and backups on
- Firewall allows 22, 80, and 443
- Caddy terminates TLS for `muster.asymbl.app` and reverse-proxies `127.0.0.1:3002`
- MongoDB 7 stores data on the volume. The database name is `muster`

DNS: an `A` record `muster` pointing at the droplet, TTL 300. If the zone is on Cloudflare, the record is DNS-only so Caddy can issue the certificate.

## Environment

`/opt/muster/.env` holds at least:

- `CLIENT_ID`, `CLIENT_SECRET`
- `MICROSOFT_CLIENT_ID`, `MICROSOFT_CLIENT_SECRET`
- `ENCRYPTION_KEY`, `SESSION_SECRET`
- `MONGODB_URI=mongodb://mongo:27017`
- `MONGO_DB_NAME=muster`
- `BASE_URL=https://muster.asymbl.app`
- `CORS_ORIGINS=https://muster.asymbl.app`
- `MUSTER_IMAGE` set by the deploy script

Listmonk and Gmail stay off unless their variables are set. There is no Stripe, ad, Discord, or Slack configuration.

## Backup and restore

Nightly, on the droplet:

```
docker compose -f /opt/muster/compose.prod.yaml exec -T mongo \
  mongodump --db=muster --archive | gzip > /mnt/muster_data/backups/muster-$(date +%F).gz
```

Keep 14 days. Restore with:

```
gunzip -c /mnt/muster_data/backups/muster-YYYY-MM-DD.gz | \
  docker compose -f /opt/muster/compose.prod.yaml exec -T mongo \
  mongorestore --drop --archive --nsInclude='muster.*'
```

Droplet backups are also enabled in DigitalOcean. A Spaces bucket is not part of this setup.
