#!/bin/sh
# Decompile vernette + SagerNet sing-geosite catalogs from defaults.json.

set -eu

DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)

"$DIR/decompile-vernette-catalog.sh"
"$DIR/decompile-sagernet-catalog.sh"
