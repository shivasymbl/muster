#!/bin/bash
# Daily Mongo dump -> local volume + R2 (muster-backups). Heartbeat only on full success,
# so UptimeRobot monitor 804387855 alerts (Slack + email) when a run fails or never happens.
set -euo pipefail
dir=/mnt/muster_data/backups
out="$dir/muster-$(date -u +%F-%H%M).gz"
docker exec -i muster-mongo-1 mongodump --db=muster --archive | gzip > "$out.tmp"
mv "$out.tmp" "$out"
rclone copy "$out" r2:muster-backups/   # copy, never sync: bucket has Object Lock
find "$dir" -name "muster-*.gz" -mtime +14 -delete
curl -fsS -m 30 -o /dev/null "$(cat "$HOME/.config/muster-backup-heartbeat")"
