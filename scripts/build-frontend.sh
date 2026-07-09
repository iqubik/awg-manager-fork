#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT/frontend"

RUNTIME_OUT="$PROJECT_ROOT/frontend/static/iq-i18n-runtime.min.js"
RUNTIME_BUILDER="${IQ_I18N_RUNTIME_BUILDER:-}"
PRIVATE_RUNTIME_BUILT=0

cleanup_private_runtime() {
	if [[ "$PRIVATE_RUNTIME_BUILT" == "1" ]]; then
		echo "Restoring public IQ i18n runtime in frontend/static..."
		git -C "$PROJECT_ROOT" checkout -- frontend/static/iq-i18n-runtime.min.js 2>/dev/null || true
		rm -f "$PROJECT_ROOT/frontend/static/iq-i18n-runtime.min.js.map"
		rm -f "$PROJECT_ROOT/frontend/static/iq-i18n-runtime.min.js.LICENSE.txt"
	fi
}

trap cleanup_private_runtime EXIT

if [[ -z "$RUNTIME_BUILDER" && -n "${IQ_I18N_PRIVATE_DIR:-}" ]]; then
	RUNTIME_BUILDER="${IQ_I18N_PRIVATE_DIR%/}/scripts/build-runtime.mjs"
fi

if [[ -n "$RUNTIME_BUILDER" && -f "$RUNTIME_BUILDER" ]]; then
	echo "Building private IQ i18n runtime..."
	node "$RUNTIME_BUILDER" \
		--out "$RUNTIME_OUT" \
		--languages "${IQ_I18N_LANGUAGES:-en,tr,fa,zh}" \
		--primary-language "${IQ_I18N_PRIMARY_LANGUAGE:-en}"
	PRIVATE_RUNTIME_BUILT=1
elif [[ "${IQ_I18N_REQUIRE_PRIVATE:-0}" == "1" ]]; then
	echo "ERROR: private IQ i18n runtime is required but builder was not found" >&2
	echo "Checked builder path: ${RUNTIME_BUILDER:-<empty>}" >&2
	exit 1
else
	echo "Using bundled public IQ i18n runtime"
fi

echo "Installing dependencies..."
npm install

echo "Building frontend..."
npm run build

runtime_asset=""
for candidate in \
	"build/iq-i18n-runtime.min.js" \
	"build/iq-i18n-runtime.min.js.gz" \
	".svelte-kit/output/client/iq-i18n-runtime.min.js"
do
	if [[ -s "$candidate" ]]; then
		runtime_asset="$candidate"
		break
	fi
done

if [[ -z "$runtime_asset" ]]; then
	echo "ERROR: iq-i18n-runtime.min.js is missing from frontend build output" >&2
	exit 1
fi

if [[ "$runtime_asset" == *.gz ]]; then
	runtime_source="$(gzip -dc "$runtime_asset")"
else
	runtime_source="$(cat "$runtime_asset")"
fi

if [[ -z "${runtime_source//[[:space:]]/}" ]]; then
	echo "ERROR: iq-i18n-runtime.min.js is empty in build output" >&2
	exit 1
fi

runtime_head="$(printf '%s' "$runtime_source" | tr '[:upper:]' '[:lower:]' | sed 's/^[[:space:]]*//' | head -c 64)"
if [[ "$runtime_head" == "<!doctype"* ]] || [[ "$runtime_head" == "<html"* ]] || [[ "$runtime_head" == "<"* ]]; then
	echo "ERROR: iq-i18n-runtime.min.js resolved to HTML fallback in build output" >&2
	exit 1
fi

if [[ "${IQ_I18N_REQUIRE_PRIVATE:-0}" == "1" ]]; then
	if printf '%s' "$runtime_source" | grep -q 'public-noop'; then
		echo "ERROR: private IQ i18n runtime required, but public-noop was built" >&2
		exit 1
	fi

	if ! printf '%s' "$runtime_source" | grep -q 'private-overlay'; then
		echo "ERROR: private IQ i18n runtime required, but private-overlay marker is missing" >&2
		exit 1
	fi
fi

echo "Frontend build complete: frontend/build/"
