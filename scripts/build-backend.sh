#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Use VERSION env if set, otherwise read from VERSION file
VERSION="${VERSION:-$(cat "$PROJECT_ROOT/VERSION" 2>/dev/null || echo "dev")}"

# ENTWARE_ARCH carries the awg-manager arch key (e.g. "mipsel-3.4").
# Set by build-ipk.sh; defaults to empty for direct invocations.
ENTWARE_ARCH="${ENTWARE_ARCH:-}"

# Architecture (default: mipsle for MT7621)
ARCH="${1:-mipsle}"

cd "$PROJECT_ROOT"

if [[ ! -f "$PROJECT_ROOT/frontend/build/index.html.gz" && ! -f "$PROJECT_ROOT/frontend/build/index.html" ]]; then
    echo "ERROR: Missing frontend/build/index.html(.gz)"
    echo "Run scripts/build-frontend.sh first."
    exit 1
fi

mkdir -p build/bin

# All targets build with the system Go toolchain. The old Go 1.23 pin for
# MIPS assumed Keenetic routers run kernel 3.4 and Go 1.24+ needs >= 3.17;
# both premises are false — the fleet runs kernel 4.9-ndm (all five Keenetic
# kernel-ABI groups), and Go 1.24+ officially requires only kernel 3.2+.
# sing-box for the same routers is already built with the system Go.
GO_CMD="go"

# vendor/ contains pre-built binaries and kernel modules (not Go vendor).
# Use -mod=mod so Go ignores it and fetches deps from go.sum.
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-mod=mod"

echo "Building awg-manager $VERSION for $ARCH ($($GO_CMD version))..."

LD_FLAGS="-s -w -X main.version=${VERSION} -X main.buildArch=${ENTWARE_ARCH}"

DEFAULT_AWG_RELEASE_BASE_URL="https://github.com/iqubik/awg-manager-fork/releases/download/iq-latest"
EFFECTIVE_AWG_RELEASE_BASE_URL="${AWG_RELEASE_BASE_URL:-$DEFAULT_AWG_RELEASE_BASE_URL}"

echo "Embedding AWG release base URL: ${EFFECTIVE_AWG_RELEASE_BASE_URL}"

LD_FLAGS="${LD_FLAGS} -X github.com/hoaxisr/awg-manager/internal/updater.releaseBaseURL=${EFFECTIVE_AWG_RELEASE_BASE_URL}"

case "$ARCH" in
    mipsle|mipsel)
        GOOS=linux GOARCH=mipsle GOMIPS=softfloat CGO_ENABLED=0 \
            $GO_CMD build -ldflags="$LD_FLAGS" \
            -tags embed_frontend \
            -o build/bin/awg-manager ./cmd/awg-manager
        ;;
    mips)
        GOOS=linux GOARCH=mips GOMIPS=softfloat CGO_ENABLED=0 \
            $GO_CMD build -ldflags="$LD_FLAGS" \
            -tags embed_frontend \
            -o build/bin/awg-manager ./cmd/awg-manager
        ;;
    arm64|aarch64)
        GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
            $GO_CMD build -ldflags="$LD_FLAGS" \
            -tags embed_frontend \
            -o build/bin/awg-manager ./cmd/awg-manager
        ;;
    *)
        echo "Unknown architecture: $ARCH"
        echo "Supported: mipsle, mips, arm64"
        exit 1
        ;;
esac

echo "Backend build complete: build/bin/awg-manager"
ls -la build/bin/awg-manager
