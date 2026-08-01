#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
VERSION="${1:-1.0.5}"
OUTPUT="${PROJECT_ROOT}/dist/yinqiao-${VERSION}.fpk"
WORK_DIR="$(mktemp -d /tmp/yinqiao-build.XXXXXX)"

cleanup() {
    chmod -R u+rwX "${WORK_DIR}" 2>/dev/null || true
    rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

mkdir -p "${WORK_DIR}/app" "${WORK_DIR}/package" "${PROJECT_ROOT}/dist"

cd "${PROJECT_ROOT}/api"
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' \
    -o "${WORK_DIR}/app/yinqiao-api" ./cmd/yinqiao-api

cd "${PROJECT_ROOT}/service"
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' \
    -o "${WORK_DIR}/app/yinqiao-service" ./cmd/yinqiao-service

mkdir -p "${WORK_DIR}/app/ui/images"
cp "${PROJECT_ROOT}/package-src/app/ui/config" "${WORK_DIR}/app/ui/config"
cp "${PROJECT_ROOT}/package-src/ICON.PNG" "${WORK_DIR}/app/ui/images/icon_64.png"
cp "${PROJECT_ROOT}/package-src/ICON_256.PNG" "${WORK_DIR}/app/ui/images/icon_256.png"

mkdir -p "${WORK_DIR}/app/docs"
cp "${PROJECT_ROOT}/LICENSE" "${PROJECT_ROOT}/THIRD_PARTY_NOTICES.md" \
    "${WORK_DIR}/app/docs/"

cp "${PROJECT_ROOT}/package-src/ICON.PNG" "${PROJECT_ROOT}/package-src/ICON_256.PNG" "${WORK_DIR}/package/"
cp "${PROJECT_ROOT}/package-src/manifest" "${WORK_DIR}/package/manifest"
cp -R "${PROJECT_ROOT}/package-src/cmd" "${PROJECT_ROOT}/package-src/config" \
    "${PROJECT_ROOT}/package-src/i18n" "${PROJECT_ROOT}/package-src/wizard" "${WORK_DIR}/package/"

chmod 755 "${WORK_DIR}/app/yinqiao-api" "${WORK_DIR}/app/yinqiao-service" "${WORK_DIR}/package/cmd/"*
chmod -R u+rwX,go+rX "${WORK_DIR}/app" "${WORK_DIR}/package"

COPYFILE_DISABLE=1 tar -czf "${WORK_DIR}/package/app.tgz" -C "${WORK_DIR}/app" .
if command -v md5sum >/dev/null 2>&1; then
    APP_CHECKSUM="$(md5sum "${WORK_DIR}/package/app.tgz" | awk '{print $1}')"
else
    APP_CHECKSUM="$(md5 -q "${WORK_DIR}/package/app.tgz")"
fi

awk -v version="${VERSION}" -v checksum="${APP_CHECKSUM}" '
    /^version[[:space:]]*=/ { print "version=\"" version "\""; next }
    /^checksum[[:space:]]*=/ { next }
    /^source[[:space:]]*=/ { print; print "checksum=\"" checksum "\""; next }
    { print }
' "${WORK_DIR}/package/manifest" >"${WORK_DIR}/package/manifest.next"
mv "${WORK_DIR}/package/manifest.next" "${WORK_DIR}/package/manifest"

COPYFILE_DISABLE=1 tar -czf "${OUTPUT}" -C "${WORK_DIR}/package" .
shasum -a 256 "${OUTPUT}" >"${OUTPUT}.sha256"

echo "Built ${OUTPUT}"
cat "${OUTPUT}.sha256"
