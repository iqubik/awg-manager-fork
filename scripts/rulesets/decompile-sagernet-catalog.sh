#!/bin/sh
# Decompile sing-geosite .srs rule-sets referenced in internal/presets/defaults.json.
#
# By default skips very large category lists (google, meta, ads, …).
# Set DECOMPILE_LARGE=1 to attempt those too (slow, needs RAM).
#
# Usage:
#   SINGBOX=build/tools/sing-box ./decompile-sagernet-catalog.sh
#   DECOMPILE_LARGE=1 ./decompile-sagernet-catalog.sh

set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
OUT="$ROOT/internal/presets/decompiled/sagernet"
BIN=${SINGBOX:-$ROOT/build/tools/sing-box}
DECOMPILE="$ROOT/scripts/rulesets/decompile-ruleset.sh"
DEFAULTS="$ROOT/internal/presets/defaults.json"

mkdir -p "$OUT"
[ -x "$BIN" ] || { echo "sing-box not found: $BIN" >&2; exit 1; }
[ -f "$DECOMPILE" ] || { echo "missing $DECOMPILE" >&2; exit 1; }
[ -f "$DEFAULTS" ] || { echo "missing $DEFAULTS" >&2; exit 1; }

export ROOT OUT DECOMPILE LARGE="${DECOMPILE_LARGE:-0}"

pairs=$(node <<'NODE'
const fs = require('fs');
const path = require('path');

const root = process.env.ROOT;
const skipDefault = new Set([
  'geosite-category-ads-all',
  'geosite-category-porn',
  'geosite-category-games',
  'geosite-category-media',
  'geosite-category-ai-!cn',
  'geosite-google',
  'geosite-apple',
  'geosite-microsoft',
  'geosite-meta',
  'geosite-facebook',
  'geosite-aws',
  'geosite-cloudflare',
]);
const skip = process.env.LARGE === '1' ? new Set() : skipDefault;

const presets = JSON.parse(fs.readFileSync(path.join(root, 'internal/presets/defaults.json'), 'utf8'));
const seen = new Map();

for (const p of presets) {
  const sb = p.engines?.singbox;
  if (!sb?.ruleSets) continue;
  for (const rs of sb.ruleSets) {
    const url = rs.url || '';
    if (!url.includes('sing-geosite')) continue;
    const tag = rs.tag || path.basename(url).replace(/\.srs$/, '');
    if (!tag || skip.has(tag)) continue;
    if (!seen.has(tag)) seen.set(tag, url);
  }
}

for (const [tag, url] of [...seen.entries()].sort((a, b) => a[0].localeCompare(b[0]))) {
  process.stdout.write(`${tag}\t${url}\n`);
}
NODE
)

if [ -z "$pairs" ]; then
	echo "no sing-geosite rule-sets to decompile" >&2
	exit 0
fi

ok=0
fail=0
IFS='
'
# shellcheck disable=SC2086
set -- $pairs
IFS='	'

for line in "$@"; do
	IFS='	'
	# shellcheck disable=SC2086
	set -- $line
	tag=$1
	url=$2
	IFS='
'
	out="$OUT/${tag}.json"
	printf 'decompile %s ... ' "$tag"
	if SINGBOX="$BIN" "$DECOMPILE" "$url" "$out" >/dev/null 2>&1; then
		printf 'ok\n'
		ok=$((ok + 1))
	else
		printf 'FAIL\n' >&2
		fail=$((fail + 1))
	fi
done

printf 'done: %d ok, %d failed -> %s\n' "$ok" "$fail" "$OUT"
if [ "${DECOMPILE_LARGE:-0}" != "1" ]; then
	printf 'tip: DECOMPILE_LARGE=1 includes google/meta/category-* (heavy)\n' >&2
fi
