#!/usr/bin/env bash
# Грязный dev-деплой с macOS: собрать awg-manager под роутер и залить по SSH.
# По умолчанию заливка через scp -O (legacy / rcp-протокол через ssh — без SFTP-подсистемы).
#
# Переменные окружения (все опциональны):
#   DEV_HOST, DEV_SSH_PORT, DEV_USER, DEV_REMOTE_BIN, DEV_ARCH
#   DEV_SKIP_RESTART, DEV_SKIP_FRONTEND, DEV_BACKEND_ONLY, DEV_FORCE_NPM_INSTALL
#   DEV_PASSWORD      пароль SSH (лучше ключи; иначе --password / sshpass)
#   DEV_SSH_OPTS, DEV_SSH_STDIN, DEV_USE_SFTP_SCP
#
# Примеры:
#   ./scripts/dev/macos_build.sh arm64 --version 2.11.2+r71
#   ./scripts/dev/macos_build.sh --password 'secret' arm64
#   ./scripts/dev/macos_build.sh --backend-only   # только Go + деплой (как раньше)
#   ./scripts/dev/macos_build.sh --skip-frontend  # то же (нужен frontend/build)
#   DEV_SKIP_RESTART=1 ./scripts/dev/macos_build.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
BUILD_BACKEND="$PROJECT_ROOT/scripts/build-backend.sh"
FRONTEND_DIR="$PROJECT_ROOT/frontend"
FRONTEND_INDEX="$FRONTEND_DIR/build/index.html"
FRONTEND_BUILD_STAMP="$FRONTEND_DIR/.macos-frontend-build.sha256"
DATA_DIR_REMOTE="${DEV_DATA_DIR:-/opt/etc/awg-manager}"
LOCAL_BIN="$PROJECT_ROOT/build/bin/awg-manager"
VERSION_FILE="$PROJECT_ROOT/VERSION"
SKIP_FRONTEND=0
# Можно задать до вызова: DEV_PASSWORD=… ./scripts/dev/macos_build.sh
DEV_PASSWORD="${DEV_PASSWORD:-}"

DEV_HOST="${DEV_HOST:-192.168.1.1}"
DEV_SSH_PORT="${DEV_SSH_PORT:-222}"
DEV_USER="${DEV_USER:-root}"
DEV_REMOTE_BIN="${DEV_REMOTE_BIN:-/opt/bin/awg-manager}"
DEV_ARCH="${DEV_ARCH:-arm64}"
BUILD_VERSION=""

usage() {
	cat <<EOF
Usage: $(basename "$0") [ARCH] [--version VER] [--password PASS] [--backend-only]

  ARCH              mipsle | mips | arm64 (default: \$DEV_ARCH or arm64)
  --version VER     ldflags main.version (default: VERSION in repo root)
  --password PASS   SSH/scp password (needs sshpass: brew install hudochenkov/sshpass/sshpass)
  --backend-only    только cross-build Go и деплой, без npm frontend (нужен frontend/build)
  --skip-frontend   синоним --backend-only

Environment: DEV_HOST, DEV_PASSWORD, DEV_ARCH, DEV_SKIP_RESTART, ...
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		--version)
			[[ $# -ge 2 ]] || {
				echo "error: --version requires a value" >&2
				exit 1
			}
			BUILD_VERSION="$2"
			shift 2
			;;
		--password)
			[[ $# -ge 2 ]] || {
				echo "error: --password requires a value" >&2
				exit 1
			}
			DEV_PASSWORD="$2"
			shift 2
			;;
		--backend-only | --skip-frontend)
			SKIP_FRONTEND=1
			shift
			;;
		-h | --help)
			usage
			exit 0
			;;
		-*)
			echo "error: unknown option $1" >&2
			usage >&2
			exit 1
			;;
		*)
			DEV_ARCH="$1"
			shift
			;;
	esac
done

if [[ -z "$BUILD_VERSION" ]]; then
	if [[ -f "$VERSION_FILE" ]]; then
		BUILD_VERSION="$(tr -d '[:space:]' <"$VERSION_FILE")"
	else
		BUILD_VERSION="dev"
	fi
fi

export VERSION="$BUILD_VERSION"

if [[ -n "$DEV_PASSWORD" ]] && ! command -v sshpass >/dev/null 2>&1; then
	echo "error: --password requires sshpass (brew install hudochenkov/sshpass/sshpass)" >&2
	exit 1
fi

dev_ssh() {
	if [[ -n "$DEV_PASSWORD" ]]; then
		# shellcheck disable=SC2086
		sshpass -e ssh ${DEV_SSH_OPTS:-} "$@"
	else
		if [[ -n "${DEV_SSH_OPTS:-}" ]]; then
			# shellcheck disable=SC2086
			ssh ${DEV_SSH_OPTS} "$@"
		else
			ssh "$@"
		fi
	fi
}

dev_scp() {
	if [[ -n "$DEV_PASSWORD" ]]; then
		# shellcheck disable=SC2086
		sshpass -e scp ${DEV_SSH_OPTS:-} "$@"
	else
		if [[ -n "${DEV_SSH_OPTS:-}" ]]; then
			# shellcheck disable=SC2086
			scp ${DEV_SSH_OPTS} "$@"
		else
			scp "$@"
		fi
	fi
}

dev_ssh_capture() {
	if [[ -n "$DEV_PASSWORD" ]]; then
		export SSHPASS="$DEV_PASSWORD"
	fi
	dev_ssh "$@"
}

# --- frontend: hash src → skip npm run build if unchanged ---
dev_frontend_sources_hash() {
	{
		[[ -f "$FRONTEND_DIR/package-lock.json" ]] && shasum -a 256 "$FRONTEND_DIR/package-lock.json"
		for f in \
			"$FRONTEND_DIR/package.json" \
			"$FRONTEND_DIR/svelte.config.js" \
			"$FRONTEND_DIR/vite.config.ts" \
			"$FRONTEND_DIR/tsconfig.json"; do
			[[ -f "$f" ]] && shasum -a 256 "$f"
		done
		if [[ -d "$FRONTEND_DIR/src" ]]; then
			find "$FRONTEND_DIR/src" -type f 2>/dev/null | sort | while read -r f; do
				shasum -a 256 "$f"
			done
		fi
		if [[ -d "$FRONTEND_DIR/static" ]]; then
			find "$FRONTEND_DIR/static" -type f 2>/dev/null | sort | while read -r f; do
				shasum -a 256 "$f"
			done
		fi
	} | shasum -a 256 | awk '{print $1}'
}

dev_npm_install() {
	if (cd "$FRONTEND_DIR" && npm ci); then
		return 0
	fi
	echo "==> npm ci failed, removing frontend/node_modules and retrying once..."
	rm -rf "$FRONTEND_DIR/node_modules"
	cd "$FRONTEND_DIR" && npm ci
}

dev_frontend_deps_ready() {
	[[ -e "$FRONTEND_DIR/node_modules/.bin/svelte-kit" ]]
}

dev_maybe_npm_install() {
	if [[ -n "${DEV_FORCE_NPM_INSTALL:-}" ]]; then
		echo "==> npm ci (DEV_FORCE_NPM_INSTALL)"
		dev_npm_install
	elif [[ ! -d "$FRONTEND_DIR/node_modules" ]] || [[ -z "$(ls -A "$FRONTEND_DIR/node_modules" 2>/dev/null || true)" ]]; then
		echo "==> npm ci (node_modules missing or empty)"
		dev_npm_install
	elif ! dev_frontend_deps_ready; then
		echo "==> npm ci (node_modules incomplete — нет .bin/svelte-kit)"
		dev_npm_install
	else
		echo "==> npm ci skipped (node_modules OK)"
	fi
}

dev_build_frontend() {
	dev_maybe_npm_install

	local src_hash stored_hash=""
	src_hash="$(dev_frontend_sources_hash)"
	if [[ -f "$FRONTEND_BUILD_STAMP" ]]; then
		stored_hash="$(tr -d '[:space:]' <"$FRONTEND_BUILD_STAMP")"
	fi

	if [[ -f "$FRONTEND_INDEX" && "$src_hash" == "$stored_hash" ]]; then
		echo "==> npm run build skipped (frontend sources unchanged)"
		return 0
	fi

	echo "==> npm run build"
	if (cd "$FRONTEND_DIR" && npm run build); then
		printf '%s\n' "$src_hash" >"$FRONTEND_BUILD_STAMP"
		return 0
	fi
	echo "==> npm run build failed, clean npm ci and retry once..."
	rm -rf "$FRONTEND_DIR/node_modules"
	dev_npm_install
	(cd "$FRONTEND_DIR" && npm run build)
	printf '%s\n' "$(dev_frontend_sources_hash)" >"$FRONTEND_BUILD_STAMP"
}

dev_upload_via_ssh_stdin() {
	local REMOTE_BIN_Q
	REMOTE_BIN_Q="$(printf '%q' "${DEV_REMOTE_BIN}")"
	dev_ssh -p "${DEV_SSH_PORT}" "${REMOTE_SPEC}" \
		"cat > ${REMOTE_BIN_Q}.new && chmod +x ${REMOTE_BIN_Q}.new && mv -f ${REMOTE_BIN_Q}.new ${REMOTE_BIN_Q}" \
		<"$LOCAL_BIN"
}

# После failed start: пробный запуск, хвост логов, статус init.
dev_diagnose_start_failure() {
	local REMOTE_BIN_Q DATA_Q
	REMOTE_BIN_Q="$(printf '%q' "${DEV_REMOTE_BIN}")"
	DATA_Q="$(printf '%q' "${DATA_DIR_REMOTE}")"

	echo ""
	echo "==> диагностика: AWG Manager не поднялся"
	dev_ssh_capture -p "${DEV_SSH_PORT}" "${REMOTE_SPEC}" bash -s <<EOF
set +e
echo "--- uname ---"
uname -m
echo "--- binary ---"
ls -la ${REMOTE_BIN_Q} 2>&1
echo "--- пробный запуск (до 5 с, stderr) ---"
( ${REMOTE_BIN_Q} -data-dir ${DATA_Q} 2>&1 & pid=\$!
  sleep 5
  kill \$pid 2>/dev/null
  wait \$pid 2>/dev/null
  true )
echo "--- S99awg-manager status ---"
/opt/etc/init.d/S99awg-manager status 2>&1
echo "--- /opt/var/log (последние 30 строк по файлам awg*) ---"
for f in /opt/var/log/awg* /opt/var/log/*awg*; do
  [ -f "\$f" ] || continue
  echo ">>> \$f"
  tail -n 30 "\$f" 2>/dev/null
done
echo "--- data dir ---"
ls -la ${DATA_Q} 2>&1 | head -20
EOF
	echo "==> подсказка: смотри stderr пробного запуска выше (panic / Failed to load)"
}

dev_remote_restart() {
	local REMOTE_BIN_Q restart_out
	REMOTE_BIN_Q="$(printf '%q' "${DEV_REMOTE_BIN}")"

	echo ""
	echo "==> ребут awg-manager (DEV_SKIP_RESTART=1 чтобы отключить)"
	restart_out="$(dev_ssh_capture -p "${DEV_SSH_PORT}" "${REMOTE_SPEC}" \
		"/opt/etc/init.d/S99awg-manager restart 2>&1; ${REMOTE_BIN_Q} --service restart 2>&1" || true)"
	printf '%s\n' "$restart_out"

	if printf '%s\n' "$restart_out" | grep -qiE 'failed to start|не запуст'; then
		dev_diagnose_start_failure
		return 1
	fi
	return 0
}

# --- main ---
cd "$PROJECT_ROOT"

if [[ -n "$DEV_PASSWORD" ]]; then
	export SSHPASS="$DEV_PASSWORD"
fi

if [[ "$SKIP_FRONTEND" -eq 1 || -n "${DEV_SKIP_FRONTEND:-}" || -n "${DEV_BACKEND_ONLY:-}" ]]; then
	if [[ ! -f "$FRONTEND_INDEX" ]]; then
		echo "error: missing $FRONTEND_INDEX (собери frontend или запусти без --backend-only)" >&2
		exit 1
	fi
	echo "==> frontend skipped (--backend-only: только Go + деплой)"
else
	dev_build_frontend
fi

echo "==> build ($DEV_ARCH) version=$VERSION"
bash "$BUILD_BACKEND" "$DEV_ARCH"
if command -v file >/dev/null 2>&1; then
	file "$LOCAL_BIN" 2>/dev/null || true
fi

if [[ ! -x "$LOCAL_BIN" ]]; then
	echo "error: expected binary at $LOCAL_BIN" >&2
	exit 1
fi

REMOTE_SPEC="${DEV_USER}@${DEV_HOST}"
REMOTE_NEW="${DEV_REMOTE_BIN}.new"

if [[ -z "$DEV_PASSWORD" ]]; then
	echo ""
	echo "    введи пароль SSH если попросит (или --password / ключи)"
fi

if [[ -n "${DEV_SSH_STDIN:-}" ]]; then
	echo "==> заливка через ssh stdin → ${REMOTE_SPEC}:${DEV_REMOTE_BIN}"
	dev_upload_via_ssh_stdin
elif [[ -n "${DEV_USE_SFTP_SCP:-}" ]]; then
	echo "==> scp (SFTP) → ${REMOTE_SPEC}:${REMOTE_NEW}"
	dev_scp -P "${DEV_SSH_PORT}" "$LOCAL_BIN" "${REMOTE_SPEC}:${REMOTE_NEW}"
else
	echo "==> scp -O → ${REMOTE_SPEC}:${REMOTE_NEW}"
	dev_scp -O -P "${DEV_SSH_PORT}" "$LOCAL_BIN" "${REMOTE_SPEC}:${REMOTE_NEW}"
fi

if [[ -z "${DEV_SSH_STDIN:-}" ]]; then
	REMOTE_BIN_Q="$(printf '%q' "${DEV_REMOTE_BIN}")"
	REMOTE_NEW_Q="$(printf '%q' "${REMOTE_NEW}")"
	dev_ssh -p "${DEV_SSH_PORT}" "${REMOTE_SPEC}" \
		"chmod +x ${REMOTE_NEW_Q} && mv -f ${REMOTE_NEW_Q} ${REMOTE_BIN_Q}"
fi

restart_failed=0
if [[ -z "${DEV_SKIP_RESTART:-}" ]]; then
	dev_remote_restart || restart_failed=1
fi

echo ""
if [[ "$restart_failed" -eq 1 ]]; then
	echo "deploy finished with errors: ${LOCAL_BIN} (v${VERSION}) → ${REMOTE_SPEC}:${DEV_REMOTE_BIN}"
	exit 1
fi
echo "done: ${LOCAL_BIN} (v${VERSION}) → ${REMOTE_SPEC}:${DEV_REMOTE_BIN}"
