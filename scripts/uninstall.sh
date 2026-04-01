#!/usr/bin/env bash

set -euo pipefail

BIN_NAME="${BIN_NAME:-civa}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

remove_binary() {
  local target="$INSTALL_DIR/$BIN_NAME"

  if [ ! -e "$target" ]; then
    printf 'Nothing to uninstall: %s not found\n' "$target"
    return 0
  fi

  if [ -x "$target" ] && "$target" uninstall --yes >/dev/null 2>&1; then
    printf 'Removed %s via "%s uninstall --yes"\n' "$target" "$BIN_NAME"
    return 0
  fi

  if [ -w "$target" ] || [ -w "$INSTALL_DIR" ]; then
    rm -f "$target"
  elif command -v sudo >/dev/null 2>&1; then
    sudo rm -f "$target"
  else
    printf 'Error: cannot remove %s and sudo is not available\n' "$target" >&2
    exit 1
  fi

  printf 'Removed %s\n' "$target"
}

main() {
  remove_binary
}

main "$@"
