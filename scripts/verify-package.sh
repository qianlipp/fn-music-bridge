#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
FPK="${1:-${PROJECT_ROOT}/dist/yinqiao-1.0.5.fpk}"
EXPECTED_VERSION="${2:-1.0.5}"
WORK_DIR="$(mktemp -d /tmp/yinqiao-verify.XXXXXX)"

cleanup() {
    chmod -R u+rwX "${WORK_DIR}" 2>/dev/null || true
    rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

mkdir -p "${WORK_DIR}/package" "${WORK_DIR}/app"
tar -xzf "${FPK}" -C "${WORK_DIR}/package"
tar -xzf "${WORK_DIR}/package/app.tgz" -C "${WORK_DIR}/app"

grep -q '^appname="yinqiao"$' "${WORK_DIR}/package/manifest"
grep -q "^version=\"${EXPECTED_VERSION}\"$" "${WORK_DIR}/package/manifest"
grep -q '^source="thirdparty"$' "${WORK_DIR}/package/manifest"
grep -q '^install_dep_apps="trim.music"$' "${WORK_DIR}/package/manifest"
grep -q '^install_type="root"$' "${WORK_DIR}/package/manifest"
grep -q '^platform="x86"$' "${WORK_DIR}/package/manifest"
grep -q '^ctl_stop="true"$' "${WORK_DIR}/package/manifest"
grep -q '^disable_authorization_path="true"$' "${WORK_DIR}/package/manifest"
grep -q '^desktop_uidir="ui"$' "${WORK_DIR}/package/manifest"
grep -q '^desktop_applaunchname="yinqiao.main"$' "${WORK_DIR}/package/manifest"
grep -q '^service_port="14040"$' "${WORK_DIR}/package/manifest"
test -x "${WORK_DIR}/app/yinqiao-api"
test -x "${WORK_DIR}/app/yinqiao-service"
test "$(find "${WORK_DIR}/app" -mindepth 1 -maxdepth 1 -type f | wc -l | tr -d ' ')" = "2"
test ! -e "${WORK_DIR}/app/trim-music"
test ! -e "${WORK_DIR}/app/static"
test -f "${WORK_DIR}/app/ui/config"
test -f "${WORK_DIR}/app/ui/images/icon_64.png"
test -f "${WORK_DIR}/app/ui/images/icon_256.png"
test -f "${WORK_DIR}/app/docs/PRIVACY.md"
test -f "${WORK_DIR}/app/docs/SECURITY.md"
test -f "${WORK_DIR}/app/docs/DISCLAIMER.md"
test -f "${WORK_DIR}/app/docs/THIRD_PARTY_NOTICES.md"
python3 -m json.tool "${WORK_DIR}/package/config/privilege" >/dev/null
python3 -m json.tool "${WORK_DIR}/package/config/resource" >/dev/null
python3 -m json.tool "${WORK_DIR}/package/wizard/install" >/dev/null
python3 -m json.tool "${WORK_DIR}/package/wizard/uninstall" >/dev/null
grep -q '"run-as": "package"' "${WORK_DIR}/package/config/privilege"
grep -q '"username": "yinqiao"' "${WORK_DIR}/package/config/privilege"
grep -q '"groupname": "yinqiao"' "${WORK_DIR}/package/config/privilege"
python3 -m json.tool "${WORK_DIR}/app/ui/config" >/dev/null
grep -q '"yinqiao.main"' "${WORK_DIR}/app/ui/config"
grep -q '"type": "iframe"' "${WORK_DIR}/app/ui/config"
grep -q '"port": "14040"' "${WORK_DIR}/app/ui/config"
grep -q '"url": "/yinqiao/ui/"' "${WORK_DIR}/app/ui/config"
grep -q 'YINQIAO_UPSTREAM_UNIX_SOCKET="${MUSIC_SOCKET}"' "${WORK_DIR}/package/cmd/service"
grep -q 'YINQIAO_SERVICE_LISTEN="0.0.0.0:${SERVICE_PORT}"' "${WORK_DIR}/package/cmd/service"
grep -q 'YINQIAO_API_URL="http://127.0.0.1:${API_PORT}/yinqiao/v1"' "${WORK_DIR}/package/cmd/service"
grep -q 'YINQIAO_SETTINGS_FILE="${SETTINGS_FILE}"' "${WORK_DIR}/package/cmd/service"
grep -q 'YINQIAO_ENABLE_SYNOLOGY="${YINQIAO_ENABLE_SYNOLOGY:-true}"' "${WORK_DIR}/package/cmd/service"
grep -q 'YINQIAO_ENABLE_JELLYFIN="${YINQIAO_ENABLE_JELLYFIN:-true}"' "${WORK_DIR}/package/cmd/service"

EXPECTED_CHECKSUM="$(awk -F'"' '/^checksum=/{print $2}' "${WORK_DIR}/package/manifest")"
if command -v md5sum >/dev/null 2>&1; then
    ACTUAL_CHECKSUM="$(md5sum "${WORK_DIR}/package/app.tgz" | awk '{print $1}')"
else
    ACTUAL_CHECKSUM="$(md5 -q "${WORK_DIR}/package/app.tgz")"
fi
test -n "${EXPECTED_CHECKSUM}"
test "${EXPECTED_CHECKSUM}" = "${ACTUAL_CHECKSUM}"

echo "Verified: yinqiao contains its API, protocol service, normal runtime privilege, and fnOS desktop entry."
