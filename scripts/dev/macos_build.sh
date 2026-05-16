#!/usr/bin/env bash
# Грязный dev-деплой с macOS: собрать awg-manager под роутер и залить по SSH.
# По умолчанию заливка через scp -O (legacy / rcp-протокол через ssh — без SFTP-подсистемы).
# Так обходится «/opt/libexec/sftp-server: not found» у современного scp по умолчанию.
# Пароль запросит scp/ssh (интерактивно), если ключи не настроены.
#
# Переменные окружения (все опциональны):
#   DEV_HOST          хост роутера (по умолчанию 192.168.1.1)
#   DEV_SSH_PORT      порт SSH (по умолчанию 222)
#   DEV_USER          пользователь SSH (по умолчанию root)
#   DEV_REMOTE_BIN    путь к бинарнику на роутере (по умолчанию /opt/bin/awg-manager)
#   DEV_ARCH          аргумент для scripts/build-backend.sh: mipsle | mips | arm64 (по умолчанию mipsle)
#   DEV_SKIP_RESTART  любое значение — не перезапускать сервис после заливки
#                     (по умолчанию: S99awg-manager restart + awg-manager --service restart)
#   DEV_SSH_OPTS          дополнительные флаги для ssh/scp (строка), например '-o ConnectTimeout=10'
#   DEV_SSH_STDIN         если задано — заливать потоком через ssh+stdin (если нет scp -O на клиенте)
#   DEV_USE_SFTP_SCP      если задано — обычный scp (SFTP); нужен рабочий sftp-server на роутере
#
# Заливка scp всегда идёт во временный файл .new на роутере, затем chmod + mv: иначе прямое
# перезаписывание пути процесса даёт Linux «scp: ... Text file busy» (ETXTBSY).
#
# Примеры:
#   ./scripts/dev/macos_build.sh
#   DEV_HOST=192.168.1.1 DEV_SSH_PORT=222 DEV_ARCH=mipsle ./scripts/dev/macos_build.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
BUILD_BACKEND="$PROJECT_ROOT/scripts/build-backend.sh"
LOCAL_BIN="$PROJECT_ROOT/build/bin/awg-manager"

DEV_HOST="${DEV_HOST:-192.168.1.1}"
DEV_SSH_PORT="${DEV_SSH_PORT:-222}"
DEV_USER="${DEV_USER:-root}"
DEV_REMOTE_BIN="${DEV_REMOTE_BIN:-/opt/bin/awg-manager}"
DEV_ARCH="${DEV_ARCH:-arm64}"

dev_ssh() {
	if [[ -n "${DEV_SSH_OPTS:-}" ]]; then
		# shellcheck disable=SC2086
		ssh ${DEV_SSH_OPTS} "$@"
	else
		ssh "$@"
	fi
}

dev_scp() {
	if [[ -n "${DEV_SSH_OPTS:-}" ]]; then
		# shellcheck disable=SC2086
		scp ${DEV_SSH_OPTS} "$@"
	else
		scp "$@"
	fi
}

# Заливка без SFTP: поток в удалённый cat (атомарная подмена через .new).
dev_upload_via_ssh_stdin() {
	local REMOTE_BIN_Q
	REMOTE_BIN_Q="$(printf '%q' "${DEV_REMOTE_BIN}")"
	dev_ssh -p "${DEV_SSH_PORT}" "${REMOTE_SPEC}" \
		"cat > ${REMOTE_BIN_Q}.new && chmod +x ${REMOTE_BIN_Q}.new && mv -f ${REMOTE_BIN_Q}.new ${REMOTE_BIN_Q}" \
		<"$LOCAL_BIN"
}

cd "$PROJECT_ROOT"

echo "==> build ($DEV_ARCH)"
bash "$BUILD_BACKEND" "$DEV_ARCH"

if [[ ! -x "$LOCAL_BIN" ]]; then
	echo "error: expected binary at $LOCAL_BIN" >&2
	exit 1
fi

REMOTE_SPEC="${DEV_USER}@${DEV_HOST}"
REMOTE_NEW="${DEV_REMOTE_BIN}.new"

echo ""
echo "    введи пароль если попросит"
if [[ -n "${DEV_SSH_STDIN:-}" ]]; then
	echo "==> заливка через ssh stdin → ${REMOTE_SPEC}:${DEV_REMOTE_BIN} (port ${DEV_SSH_PORT}, DEV_SSH_STDIN)"
	dev_upload_via_ssh_stdin
elif [[ -n "${DEV_USE_SFTP_SCP:-}" ]]; then
	echo "==> scp (SFTP) → ${REMOTE_SPEC}:${REMOTE_NEW} затем mv (port ${DEV_SSH_PORT}, DEV_USE_SFTP_SCP)"
	dev_scp -P "${DEV_SSH_PORT}" "$LOCAL_BIN" "${REMOTE_SPEC}:${REMOTE_NEW}"
else
	echo "==> scp -O (legacy, без SFTP) → ${REMOTE_SPEC}:${REMOTE_NEW} затем mv (port ${DEV_SSH_PORT})"
	dev_scp -O -P "${DEV_SSH_PORT}" "$LOCAL_BIN" "${REMOTE_SPEC}:${REMOTE_NEW}"
fi

if [[ -z "${DEV_SSH_STDIN:-}" ]]; then
	REMOTE_BIN_Q="$(printf '%q' "${DEV_REMOTE_BIN}")"
	REMOTE_NEW_Q="$(printf '%q' "${REMOTE_NEW}")"
	dev_ssh -p "${DEV_SSH_PORT}" "${REMOTE_SPEC}" \
		"chmod +x ${REMOTE_NEW_Q} && mv -f ${REMOTE_NEW_Q} ${REMOTE_BIN_Q}"
fi

if [[ -z "${DEV_SKIP_RESTART:-}" ]]; then
	REMOTE_BIN_Q="$(printf '%q' "${DEV_REMOTE_BIN}")"
	echo ""
	echo "==> ребут awg-manager: init + --service restart (DEV_SKIP_RESTART=1 чтобы отключить)"
	dev_ssh -p "${DEV_SSH_PORT}" "${REMOTE_SPEC}" \
		"/opt/etc/init.d/S99awg-manager restart 2>/dev/null || true; ${REMOTE_BIN_Q} --service restart 2>/dev/null || true"
fi

echo ""
echo "done: ${LOCAL_BIN} → ${REMOTE_SPEC}:${DEV_REMOTE_BIN}"
