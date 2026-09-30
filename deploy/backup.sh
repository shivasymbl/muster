#!/bin/sh
set -eu
mkdir -p /mnt/muster_data/backups
stamp=$(date +%F)
docker compose -f /opt/muster/compose.yaml exec -T mongo \
  mongodump --db=muster --archive | gzip > "/mnt/muster_data/backups/muster-${stamp}.gz"
find /mnt/muster_data/backups -name 'muster-*.gz' -mtime +14 -delete
