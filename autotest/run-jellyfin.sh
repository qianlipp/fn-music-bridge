#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

: "${JELLYFIN_URL:?Set JELLYFIN_URL, for example https://music.example.com}"
: "${JELLYFIN_USER:?Set JELLYFIN_USER}"
: "${JELLYFIN_PASSWORD:?Set JELLYFIN_PASSWORD}"

exec python3 "$SCRIPT_DIR/jellyfin_autotest.py" \
  --url "$JELLYFIN_URL" \
  --username "$JELLYFIN_USER" \
  --password "$JELLYFIN_PASSWORD" \
  "$@"
