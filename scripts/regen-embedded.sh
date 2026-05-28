#!/usr/bin/env bash
# Regenerates internal/singbox/installer/embedded.go from pinned
# SINGBOX_VERSION by downloading binaries from the develop "latest"
# GitHub pre-release. No compilation. Writes unified-mirror URLs
# (http://repo.hoaxisr.ru/develop/singbox/<ver>/...).
#
# Usage:
#   ./scripts/regen-embedded.sh
# Requires: gh CLI authenticated, sha256sum, stat, sed, python3.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

source "$SCRIPT_DIR/singbox-version.env"
VERSION="$SINGBOX_VERSION"
EMBEDDED_GO="$PROJECT_ROOT/internal/singbox/installer/embedded.go"
ARCHES=(mipsel-3.4 mips-3.4 aarch64-3.10)
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Regenerating embedded.go for sing-box $VERSION"

# Update RequiredVersion line.
python3 - <<PY
import pathlib, re
p = pathlib.Path("$EMBEDDED_GO")
text = p.read_text()
text = re.sub(r'const RequiredVersion = "[^"]*"',
              f'const RequiredVersion = "$VERSION"', text, count=1)
p.write_text(text)
PY

# For each arch: download, compute, rewrite map entry.
for arch in "${ARCHES[@]}"; do
    asset="sing-box-${VERSION}-${arch}"
    dest="$TMP/$asset"

    echo "  Downloading $asset from develop 'latest' release..."
    gh release download latest \
        --repo hoaxisr/awg-manager \
        --pattern "$asset" \
        --dir "$TMP"
    if [[ ! -f "$dest" ]]; then
        echo "ERROR: $asset not present in develop 'latest' release. Run develop CI first." >&2
        exit 1
    fi

    sha="$(sha256sum "$dest" | awk '{print $1}')"
    size="$(stat -c '%s' "$dest")"
    url="http://repo.hoaxisr.ru/develop/singbox/${VERSION}/${asset}"

    URL="$url" SHA="$sha" SIZE="$size" ARCH="$arch" EMBEDDED_GO="$EMBEDDED_GO" python3 - <<'PY'
import os, pathlib, re, sys
p = pathlib.Path(os.environ["EMBEDDED_GO"])
arch = os.environ["ARCH"]
text = p.read_text()
pattern = re.compile(
    rf'(\t"{re.escape(arch)}":\s*)'
    r'\{Version: RequiredVersion, URL: "[^"]*", SHA256: "[^"]*"(?:, Size: \d+)?\},'
)
replacement = (
    rf'\1{{Version: RequiredVersion, URL: "{os.environ["URL"]}", SHA256: "{os.environ["SHA"]}", Size: {os.environ["SIZE"]}}},'
)
text, n = pattern.subn(replacement, text, count=1)
if n != 1:
    sys.stderr.write(f"ERROR: failed to update embedded.go entry for {arch}\n")
    sys.exit(1)
p.write_text(text)
PY

    echo "  Updated $arch (sha=${sha:0:12}..., size=$size)"
done

gofmt -w "$EMBEDDED_GO"
echo "Done. Diff:"
git diff --stat "$EMBEDDED_GO" || true
