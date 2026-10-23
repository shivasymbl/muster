#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$ROOT"

: "${DROPLET_HOST:?Set DROPLET_HOST to the droplet IP or hostname}"
PROJECT=$(sed -n 's/.*"projectID": *"\([^"]*\)".*/\1/p' depot.json)
SHA=$(git rev-parse --short HEAD)
IMAGE="registry.depot.dev/${PROJECT}:${SHA}"

depot build . \
  --project "$PROJECT" \
  --platform linux/amd64 \
  --build-arg "VUE_APP_GOOGLE_CLIENT_ID=${GOOGLE_CLIENT_ID:-}" \
  --build-arg "VUE_APP_MICROSOFT_CLIENT_ID=${MICROSOFT_CLIENT_ID:-}" \
  --save --save-tag "$SHA" --save-tag latest

ssh "deploy@${DROPLET_HOST}" "mkdir -p /opt/muster"
scp "$ROOT/deploy/compose.prod.yaml" "deploy@${DROPLET_HOST}:/opt/muster/compose.yaml"
scp "$ROOT/deploy/backup.sh" "deploy@${DROPLET_HOST}:/opt/muster/backup.sh"
ssh "deploy@${DROPLET_HOST}" "chmod 755 /opt/muster/backup.sh"

# Short-lived project-scoped pull token; logged out again right after the pull.
depot pull-token --project "$PROJECT" | ssh "deploy@${DROPLET_HOST}" "docker login registry.depot.dev -u x-token --password-stdin >/dev/null"
ssh "deploy@${DROPLET_HOST}" "docker pull '${IMAGE}'; rc=\$?; docker logout registry.depot.dev >/dev/null; exit \$rc"
# Record the image in .env too, so a plain 'docker compose up' later keeps this version.
ssh "deploy@${DROPLET_HOST}" "sed -i 's|^MUSTER_IMAGE=.*|MUSTER_IMAGE=${IMAGE}|' /opt/muster/.env && docker compose -f /opt/muster/compose.yaml up -d"

deadline=$(( $(date +%s) + 120 ))
while [ "$(date +%s)" -lt "$deadline" ]; do
  if curl -fsS "https://muster.asymbl.app/api/health" >/dev/null; then
    ssh "deploy@${DROPLET_HOST}" "printf '%s\n' '${SHA}' > /opt/muster/.last_good_sha"
    echo "deployed ${SHA}"
    exit 0
  fi
  sleep 5
done

echo "health check failed, rolling back" >&2
PREV=$(ssh "deploy@${DROPLET_HOST}" "cat /opt/muster/.last_good_sha 2>/dev/null || true")
if [ -n "$PREV" ]; then
  PREV_IMAGE="registry.depot.dev/${PROJECT}:${PREV}"
  ssh "deploy@${DROPLET_HOST}" "sed -i 's|^MUSTER_IMAGE=.*|MUSTER_IMAGE=${PREV_IMAGE}|' /opt/muster/.env && docker compose -f /opt/muster/compose.yaml up -d"
fi
exit 1
