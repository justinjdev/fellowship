#!/usr/bin/env bash
# Thin wrapper — ensures binary exists, then execs it with all args.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="$HOME/.claude/fellowship/bin/fellowship"

"$SCRIPT_DIR/ensure-binary.sh" || exit $?

exec "$BINARY" "$@"
