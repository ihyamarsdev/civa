#!/usr/bin/env bash

set -euo pipefail

REPO="ihyamarsdev/civa"
BIN_NAME="civa"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
TMP_DIR=""

cleanup() {
  if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}

trap cleanup EXIT

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'Error: required command not found: %s\n' "$1" >&2
    exit 1
  }
}

detect_os() {
  case "$(uname -s)" in
    Linux) printf '%s' 'linux' ;;
    Darwin) printf '%s' 'darwin' ;;
    *)
      printf 'Error: unsupported operating system: %s\n' "$(uname -s)" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf '%s' 'amd64' ;;
    aarch64|arm64) printf '%s' 'arm64' ;;
    *)
      printf 'Error: unsupported architecture: %s\n' "$(uname -m)" >&2
      exit 1
      ;;
  esac
}

fetch() {
  local url="$1"
  local output="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$output"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$output" "$url"
  else
    printf '%s\n' 'Error: curl or wget is required to download civa.' >&2
    exit 1
  fi
}

release_version() {
  if [ -n "${CIVA_VERSION:-}" ]; then
    printf '%s' "$CIVA_VERSION"
    return 0
  fi

  local api_url="https://api.github.com/repos/$REPO/releases/latest"
  local metadata_file="$TMP_DIR/release.json"
  fetch "$api_url" "$metadata_file"

  python - <<'PY' "$metadata_file"
import json, sys
with open(sys.argv[1], 'r', encoding='utf-8') as fh:
    data = json.load(fh)
tag = data.get('tag_name')
if not tag:
    raise SystemExit('Error: could not determine latest release tag')
print(tag)
PY
}

install_binary() {
  local source_binary="$1"
  local destination="$INSTALL_DIR/$BIN_NAME"

  if [ -w "$INSTALL_DIR" ]; then
    install -m 755 "$source_binary" "$destination"
  elif command -v sudo >/dev/null 2>&1; then
    sudo install -m 755 "$source_binary" "$destination"
  else
    printf 'Error: cannot write to %s and sudo is not available\n' "$INSTALL_DIR" >&2
    exit 1
  fi
}

main() {
  need_cmd tar
  need_cmd python

  TMP_DIR="$(mktemp -d)"
  mkdir -p "$INSTALL_DIR"

  local os_name="$(detect_os)"
  local arch_name="$(detect_arch)"
  local version="$(release_version)"
  local archive_name="${BIN_NAME}_${os_name}_${arch_name}.tar.gz"
  local download_url="https://github.com/$REPO/releases/download/$version/$archive_name"
  local archive_path="$TMP_DIR/$archive_name"
  local extract_dir="$TMP_DIR/extract"

  printf 'Installing %s %s for %s/%s\n' "$BIN_NAME" "$version" "$os_name" "$arch_name"
  fetch "$download_url" "$archive_path"
  mkdir -p "$extract_dir"
  tar -xzf "$archive_path" -C "$extract_dir"

  if [ ! -f "$extract_dir/$BIN_NAME" ]; then
    printf 'Error: extracted archive does not contain %s\n' "$BIN_NAME" >&2
    exit 1
  fi

  install_binary "$extract_dir/$BIN_NAME"
  printf 'Installed %s to %s/%s\n' "$BIN_NAME" "$INSTALL_DIR" "$BIN_NAME"
  printf 'Run "%s help" to get started.\n' "$BIN_NAME"
}

main "$@"
