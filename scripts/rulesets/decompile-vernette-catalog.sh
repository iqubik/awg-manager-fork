#!/bin/sh
# Decompile all small vernette .srs URLs from defaults.json (skips rkn).
# Requires sing-box with `rule-set decompile` (SINGBOX or build/tools/sing-box).

set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
OUT="$ROOT/internal/presets/decompiled/vernette"
DECOMPILE="$ROOT/scripts/rulesets/decompile-ruleset.sh"
BIN=${SINGBOX:-$ROOT/build/tools/sing-box}
BASE_URL=https://github.com/vernette/rulesets/raw/master/srs

SETS="
claude
copilot
gemini
grok
openai
unavailable-in-russia
linkedin
nintendo
roblox
netflix
discord-full
instagram
telegram
tiktok
whatsapp
x
youtube
"

mkdir -p "$OUT"
[ -x "$BIN" ] || { echo "sing-box not found: $BIN" >&2; exit 1; }
[ -f "$DECOMPILE" ] || { echo "missing $DECOMPILE" >&2; exit 1; }

ok=0
fail=0
for name in $SETS; do
	url="$BASE_URL/${name}.srs"
	out="$OUT/${name}.json"
	printf 'decompile %s ... ' "$name"
	if SINGBOX="$BIN" "$DECOMPILE" "$url" "$out" >/dev/null 2>&1; then
		printf 'ok\n'
		ok=$((ok + 1))
	else
		printf 'FAIL\n' >&2
		fail=$((fail + 1))
	fi
done
printf 'done: %d ok, %d failed -> %s\n' "$ok" "$fail" "$OUT"
