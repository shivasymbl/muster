#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$ROOT"

: "${DROPLET_HOST:?Set DROPLET_HOST to the droplet IP or hostname}"
: "${DEPOT_ORG_ID:?Set DEPOT_ORG_ID}"
SHA=$(git rev-parse --short HEAD)
IMAGE="${DEPOT_ORG_ID}.registry.depot.dev/muster:${SHA}"
LATEST="${DEPOT_ORG_ID}.registry.depot.dev/muster:latest"

depot build . \
  --platform linux/amd64 \
  --build-arg "VUE_APP_GOOGLE_CLIENT_ID=${GOOGLE_CLIENT_ID:-}" \
  --build-arg "VUE_APP_MICROSOFT_CLIENT_ID=${MICROSOFT_CLIENT_ID:-}" \
  -t "$IMAGE" \
  -t "$LATEST" \
  --push

ssh "deploy@${DROPLET_HOST}" "mkdir -p /opt/muster"
scp "$ROOT/deploy/compose.prod.yaml" "deploy@${DROPLET_HOST}:/opt/muster/compose.yaml"
scp "$ROOT/deploy/backup.sh" "deploy@${DROPLET_HOST}:/opt/muster/backup.sh"
ssh "deploy@${DROPLET_HOST}" "chmod 755 /opt/muster/backup.sh"

ssh "deploy@${DROPLET_HOST}" "MUSTER_IMAGE='${IMAGE}' docker compose -f /opt/muster/compose.yaml pull && MUSTER_IMAGE='${IMAGE}' docker compose -f /opt/muster/compose.yaml up -d"

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
  PREV_IMAGE="${DEPOT_ORG_ID}.registry.depot.dev/muster:${PREV}"
  ssh "deploy@${DROPLET_HOST}" "MUSTER_IMAGE='${PREV_IMAGE}' docker compose -f /opt/muster/compose.yaml up -d"
fi
exit 1
