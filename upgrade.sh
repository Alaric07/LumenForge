#!/bin/sh

# Compatibility wrapper. The allowlisted install.sh path handles both installs and upgrades.
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec /bin/bash "$SCRIPT_DIR/install.sh" "$@"
