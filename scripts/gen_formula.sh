#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <version-tag>" >&2
  echo "example: $0 v0.1.0" >&2
  exit 1
fi

VERSION="$1"
BASE_URL="https://github.com/MiUPa/codex-notify/releases/download/${VERSION}"
CHECKSUMS_URL="${BASE_URL}/checksums.txt"

TMP_FILE="$(mktemp)"
cleanup() {
  rm -f "${TMP_FILE}"
}
trap cleanup EXIT

curl -fsSL "${CHECKSUMS_URL}" -o "${TMP_FILE}"

AMD64_SHA256="$(awk -v v="${VERSION}" '$2 ~ "codex-notify_" v "_darwin_amd64.tar.gz" { print $1 }' "${TMP_FILE}")"
ARM64_SHA256="$(awk -v v="${VERSION}" '$2 ~ "codex-notify_" v "_darwin_arm64.tar.gz" { print $1 }' "${TMP_FILE}")"

if [[ -z "${AMD64_SHA256}" || -z "${ARM64_SHA256}" ]]; then
  echo "failed to parse checksums.txt for version ${VERSION}" >&2
  exit 1
fi

cat <<EOF
class CodexNotify < Formula
  desc "macOS desktop notification bridge for Codex CLI"
  homepage "https://github.com/MiUPa/codex-notify"
  version "${VERSION#v}"
  license "Apache-2.0"

  if Hardware::CPU.arm?
    url "${BASE_URL}/codex-notify_${VERSION}_darwin_arm64.tar.gz"
    sha256 "${ARM64_SHA256}"
  else
    url "${BASE_URL}/codex-notify_${VERSION}_darwin_amd64.tar.gz"
    sha256 "${AMD64_SHA256}"
  end

  def install
    bin.install "codex-notify"
  end

  def post_install
    system_command(bin/"codex-notify", args: ["init"])
  rescue ErrorDuringExecution
    opoo "Automatic Codex notify hook setup failed. Run `codex-notify init --replace` manually."
  end

  test do
    assert_match "macOS desktop notifications for Codex CLI", shell_output("#{bin}/codex-notify help")
  end
end
EOF
