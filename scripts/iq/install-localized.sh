#!/bin/sh
set -eu

RELEASE_BASE_URL="${RELEASE_BASE_URL:-https://github.com/iqubik/awg-manager-fork/releases/download/iq-latest}"
TMP_DIR="${TMP_DIR:-/opt/tmp/awg-manager-iq-install}"

detect_arch() {
	case "$(uname -m)" in
		mipsel|mipsle)
			echo "mipsel-3.4"
			;;
		mips)
			echo "mips-3.4"
			;;
		aarch64|arm64)
			echo "aarch64-3.10"
			;;
		*)
			echo "Unsupported architecture: $(uname -m)" >&2
			exit 1
			;;
	esac
}

require_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "Missing required command: $1" >&2
		exit 1
	fi
}

require_cmd wget
require_cmd opkg

ARCH="$(detect_arch)"

rm -rf "$TMP_DIR"
mkdir -p "$TMP_DIR"

VERSION_URL="${RELEASE_BASE_URL%/}/VERSION"
VERSION_FILE="$TMP_DIR/VERSION"

echo "Downloading VERSION: $VERSION_URL"
wget -qO "$VERSION_FILE" "$VERSION_URL"

VERSION="$(tr -d '[:space:]' < "$VERSION_FILE")"
if [ -z "$VERSION" ]; then
	echo "Empty VERSION from $VERSION_URL" >&2
	exit 1
fi

PKG="awg-manager_${VERSION}_${ARCH}-kn.ipk"
PKG_URL="${RELEASE_BASE_URL%/}/${PKG}"
PKG_PATH="$TMP_DIR/$PKG"

echo "Downloading package: $PKG_URL"
wget -qO "$PKG_PATH" "$PKG_URL"

echo "Installing $PKG"
opkg install "$PKG_PATH"

echo "Installed IQ localized AWG Manager: $VERSION / $ARCH"
