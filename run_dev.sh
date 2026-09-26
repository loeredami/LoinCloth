#!/bin/bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
cd "$ROOT_DIR"

go build -o "$ROOT_DIR/loin-dev" .
exec "$ROOT_DIR/loin-dev" --cloth "$ROOT_DIR/default.cloth" "$@"
