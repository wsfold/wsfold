#!/usr/bin/env bash
set -euo pipefail

VERSION="${GORELEASER_VERSION:-2.14.3}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${ROOT_DIR}/.bin"
BIN_PATH="${BIN_DIR}/goreleaser"

os="$(uname -s)"
arch="$(uname -m)"

case "${os}" in
  Linux) platform_os="Linux" ;;
  Darwin) platform_os="Darwin" ;;
  *)
    echo "unsupported host OS for goreleaser bootstrap: ${os}" >&2
    exit 1
    ;;
esac

case "${arch}" in
  x86_64|amd64) platform_arch="x86_64" ;;
  arm64|aarch64) platform_arch="arm64" ;;
  *)
    echo "unsupported host architecture for goreleaser bootstrap: ${arch}" >&2
    exit 1
    ;;
esac

archive="goreleaser_${platform_os}_${platform_arch}.tar.gz"
url="https://github.com/goreleaser/goreleaser/releases/download/v${VERSION}/${archive}"

mkdir -p "${BIN_DIR}"

if [[ ! -x "${BIN_PATH}" ]]; then
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "${tmpdir}"' EXIT

  curl -fsSL "${url}" -o "${tmpdir}/${archive}"
  tar -xzf "${tmpdir}/${archive}" -C "${tmpdir}" goreleaser
  mv "${tmpdir}/goreleaser" "${BIN_PATH}"
  chmod +x "${BIN_PATH}"
fi

exec "${BIN_PATH}" "$@"
