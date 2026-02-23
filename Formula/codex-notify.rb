class CodexNotify < Formula
  desc "macOS desktop notification bridge for Codex CLI"
  homepage "https://github.com/MiUPa/codex-notify"
  version "0.3.1"
  license "Apache-2.0"

  if Hardware::CPU.arm?
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.3.1/codex-notify_v0.3.1_darwin_arm64.tar.gz"
    sha256 "cf679589edb67fb8ac140d813d04f8aac7fcffc0e9032fc80263ea0de7113b36"
  else
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.3.1/codex-notify_v0.3.1_darwin_amd64.tar.gz"
    sha256 "a211955c06d2774b53a94746484eb8d85ffab104fe494386c8750f1fca51f2a3"
  end

  def install
    bin.install "codex-notify"
  end

  test do
    assert_match "macOS desktop notifications for Codex CLI", shell_output("#{bin}/codex-notify help")
  end
end
