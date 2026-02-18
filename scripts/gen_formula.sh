#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <version-tag>" >&2
  echo "example: $0 v0.1.0" >&2
  exit 1
fi

VERSION="$1"
URL="https://github.com/MiUPa/codex-notify/archive/refs/tags/${VERSION}.tar.gz"

TMP_FILE="$(mktemp)"
cleanup() {
  rm -f "${TMP_FILE}"
}
trap cleanup EXIT

curl -fsSL "${URL}" -o "${TMP_FILE}"
SHA256="$(shasum -a 256 "${TMP_FILE}" | awk '{print $1}')"

cat <<EOF
class CodexNotify < Formula
  desc "macOS desktop notification bridge for Codex CLI"
  homepage "https://github.com/MiUPa/codex-notify"
  url "${URL}"
  sha256 "${SHA256}"
  license "Apache-2.0"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "."
  end

  test do
    assert_match "macOS desktop notifications for Codex CLI", shell_output("#{bin}/codex-notify help")
  end
end
EOF
