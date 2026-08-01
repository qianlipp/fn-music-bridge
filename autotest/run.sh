#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

: "${SUBSONIC_URL:?Set SUBSONIC_URL, for example https://music.example.com}"
: "${SUBSONIC_USER:?Set SUBSONIC_USER}"
: "${SUBSONIC_PASSWORD:?Set SUBSONIC_PASSWORD}"

exec python3 "$SCRIPT_DIR/subsonic_autotest.py" \
  --url "$SUBSONIC_URL" \
  --username "$SUBSONIC_USER" \
  --password "$SUBSONIC_PASSWORD" \
  "$@"
