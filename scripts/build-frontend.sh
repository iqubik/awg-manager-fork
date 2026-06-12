#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT/frontend"

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

echo "Frontend build complete: frontend/build/"
